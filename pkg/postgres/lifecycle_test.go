package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error { return f.err }

type pointerPinger struct{}

func (*pointerPinger) Ping(context.Context) error { return nil }

func TestHealthWrapsBackendErrors(t *testing.T) {
	backendErr := errors.New("connection refused")
	err := Health(context.Background(), fakePinger{err: backendErr})
	if !errors.Is(err, backendErr) || !strings.Contains(err.Error(), "database health check") {
		t.Fatalf("Health() error = %v", err)
	}
	if err := Health(context.Background(), fakePinger{}); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}

func TestHealthRejectsTypedNilPinger(t *testing.T) {
	var pinger *pointerPinger
	if err := Health(context.Background(), pinger); err == nil {
		t.Fatal("typed nil pinger must fail")
	}
}

func TestOpenRejectsUnavailableDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool, err := Open(ctx, Config{
		URL:      "postgres://postgres:postgres@127.0.0.1:1/test?sslmode=disable",
		MaxConns: 1,
	})
	if pool != nil {
		pool.Close()
		t.Fatal("Open() returned a pool for an unavailable database")
	}
	if err == nil || !strings.Contains(err.Error(), "ping database") {
		t.Fatalf("Open() error = %v, want ping database error", err)
	}
}
