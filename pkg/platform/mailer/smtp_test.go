package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func smtpTestServer(t *testing.T, failAuth bool, stall bool) (string, *tls.Config) {
	t.Helper()
	certServer := httptest.NewTLSServer(nil)
	defer certServer.Close()
	listener, e := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: certServer.TLS.Certificates, MinVersion: tls.VersionTLS12})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		if stall {
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			_, _ = io.Copy(io.Discard, conn)
			return
		}
		io.WriteString(conn, "220 localhost ESMTP\r\n")
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "EHLO"):
				io.WriteString(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(line, "AUTH"):
				if failAuth {
					io.WriteString(conn, "535 invalid credentials\r\n")
				} else {
					io.WriteString(conn, "235 authenticated\r\n")
				}
			case strings.HasPrefix(line, "DATA"):
				io.WriteString(conn, "354 continue\r\n")
				for scanner.Scan() {
					if scanner.Text() == "." {
						break
					}
				}
				io.WriteString(conn, "250 queued\r\n")
			case line == "QUIT":
				io.WriteString(conn, "221 bye\r\n")
				return
			default:
				io.WriteString(conn, "250 ok\r\n")
			}
		}
	}()
	cfg := certServer.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	return listener.Addr().String(), cfg
}
func TestSMTPTLSAuthenticationAndTimeout(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		auth, stall, untrusted bool
		timeout                time.Duration
		want                   error
	}{
		{"success", false, false, false, 5 * time.Second, nil},
		{"auth", true, false, false, 5 * time.Second, ErrAuthentication},
		{"timeout", false, true, false, 100 * time.Millisecond, ErrTimeout},
		{"certificate", false, false, true, 5 * time.Second, ErrTLS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, tlsConfig := smtpTestServer(t, tc.auth, tc.stall)
			if tc.untrusted {
				tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			}
			host, port, _ := net.SplitHostPort(addr)
			m := New(Config{Enabled: true, Host: host, Port: port, Username: "sender", Password: "password", From: "noreply@example.com", TLSMode: "tls", Timeout: tc.timeout, TLSConfig: tlsConfig}, nil, nil)
			e := m.SendCode(context.Background(), "u@example.com", "register", "123456")
			if !errors.Is(e, tc.want) {
				t.Fatalf("got %v, want %v", e, tc.want)
			}
		})
	}
}
func TestDevelopmentLogContainsNoCredentials(t *testing.T) {
	var out strings.Builder
	m := New(Config{Enabled: false}, slog.New(slog.NewTextHandler(&out, nil)), []byte(strings.Repeat("x", 32)))
	if e := m.SendCode(context.Background(), "u@example.com", "register", "123456"); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out.String(), "u@example.com") || strings.Contains(out.String(), "123456") {
		t.Fatal("secret logged")
	}
	if !strings.Contains(out.String(), "email_code") {
		t.Fatal("missing development event")
	}
}

func TestSMTPStartTLSUpgrade(t *testing.T) {
	certServer := httptest.NewTLSServer(nil)
	defer certServer.Close()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		io.WriteString(conn, "220 localhost ESMTP\r\n")
		reader := bufio.NewReader(conn)
		_, _ = reader.ReadString('\n')
		io.WriteString(conn, "250-localhost\r\n250 STARTTLS\r\n")
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) != "STARTTLS" {
			return
		}
		io.WriteString(conn, "220 begin TLS\r\n")
		secure := tls.Server(conn, &tls.Config{Certificates: certServer.TLS.Certificates, MinVersion: tls.VersionTLS12})
		defer secure.Close()
		if secure.Handshake() != nil {
			return
		}
		scanner := bufio.NewScanner(secure)
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "EHLO"):
				io.WriteString(secure, "250-localhost\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(line, "AUTH"):
				io.WriteString(secure, "235 authenticated\r\n")
			case line == "DATA":
				io.WriteString(secure, "354 continue\r\n")
				for scanner.Scan() {
					if scanner.Text() == "." {
						break
					}
				}
				io.WriteString(secure, "250 queued\r\n")
			case line == "QUIT":
				io.WriteString(secure, "221 bye\r\n")
				return
			default:
				io.WriteString(secure, "250 ok\r\n")
			}
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	m := New(Config{Enabled: true, Host: host, Port: port, Username: "sender", Password: "password", From: "noreply@example.com", TLSMode: "starttls", Timeout: time.Second, TLSConfig: certServer.Client().Transport.(*http.Transport).TLSClientConfig}, nil, nil)
	if e = m.SendCode(context.Background(), "u@example.com", "login", "123456"); e != nil {
		t.Fatal(e)
	}
	<-done
}
