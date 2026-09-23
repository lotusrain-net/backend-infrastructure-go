package mailer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
)

func plainServer(t *testing.T, reject string, response int, stall bool) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		if stall {
			_, _ = io.Copy(io.Discard, conn)
			return
		}
		fmt.Fprint(conn, "220 SMTP\r\n")
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			if reject != "" && strings.HasPrefix(line, reject) {
				fmt.Fprintf(conn, "%d private@example.com secret-diagnostic\r\n", response)
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(line, "AUTH"):
				fmt.Fprint(conn, "235 authenticated\r\n")
			case line == "DATA":
				fmt.Fprint(conn, "354 data\r\n")
				for scanner.Scan() {
					if scanner.Text() == "." {
						break
					}
				}
				fmt.Fprint(conn, "250 accepted\r\n")
			case line == "QUIT":
				fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				fmt.Fprint(conn, "250 ok\r\n")
			}
		}
	}()
	return l.Addr().String()
}

func TestPlainSMTPAndFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name, mode, stage string
		status            int
		want              error
	}{
		{"plain", "plain", "", 0, nil},
		{"no-starttls", "starttls", "", 0, iam.ErrAuthenticationUnavailable},
		{"authentication", "plain", "AUTH", 535, iam.ErrAuthenticationUnavailable},
		{"sender", "plain", "MAIL", 550, iam.ErrAuthenticationUnavailable},
		{"recipient", "plain", "RCPT", 550, iam.ErrMailRejected},
		{"content", "plain", "DATA", 554, iam.ErrMailRejected},
		{"service", "plain", "RCPT", 421, iam.ErrAuthenticationUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := plainServer(t, tc.stage, tc.status, false)
			host, port, _ := net.SplitHostPort(addr)
			var logs strings.Builder
			m := New(Config{Enabled: true, Host: host, Port: port, Username: "sender", Password: "smtp-secret", From: "sender@example.com", TLSMode: tc.mode, Timeout: time.Second}, slog.New(slog.NewTextHandler(&logs, nil)), []byte("pepper"))
			err := m.SendCode(context.Background(), "private@example.com", iam.EmailCodeLogin, "123456")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			for _, secret := range []string{"private@example.com", "123456", "smtp-secret", "secret-diagnostic"} {
				if strings.Contains(logs.String(), secret) || (err != nil && strings.Contains(err.Error(), secret)) {
					t.Fatal("sensitive SMTP detail escaped")
				}
			}
		})
	}
}

func TestExplicitPlainAuthAllowsConfiguredRemoteHostOnly(t *testing.T) {
	a := explicitPlainAuth{host: "smtp.example.com", username: "user", password: "pass"}
	name, data, err := a.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: false})
	if err != nil || name != "PLAIN" || string(data) != "\x00user\x00pass" {
		t.Fatal(name, err)
	}
	if _, _, err := a.Start(&smtp.ServerInfo{Name: "other.example.com"}); err == nil {
		t.Fatal("authenticated to another host")
	}
}

func TestSMTPCancellationClosesPendingConnection(t *testing.T) {
	addr := plainServer(t, "", 0, true)
	host, port, _ := net.SplitHostPort(addr)
	m := New(Config{Enabled: true, Host: host, Port: port, Username: "sender", Password: "secret", From: "s@example.com", TLSMode: "plain", Timeout: time.Second}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	start := time.Now()
	if err := m.SendCode(ctx, "a@example.com", iam.EmailCodeLogin, "123456"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("cancellation did not close SMTP connection")
	}
}
