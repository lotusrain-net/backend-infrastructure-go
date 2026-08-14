package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeBeginner struct {
	tx       *fakeTx
	beginErr error
}

func (f *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

type fakeTx struct {
	pgx.Tx
	commits     int
	rollbacks   int
	commitErr   error
	rollbackErr error
}

func (f *fakeTx) Commit(context.Context) error {
	f.commits++
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rollbacks++
	return f.rollbackErr
}

func TestRunInTxCommitsSuccessfulWork(t *testing.T) {
	tx := &fakeTx{}
	err := RunInTx(context.Background(), &fakeBeginner{tx: tx}, func(_ context.Context, got pgx.Tx) error {
		if got != tx {
			t.Fatal("callback received a different transaction")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RunInTx() error = %v", err)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("commits = %d, rollbacks = %d", tx.commits, tx.rollbacks)
	}
}

func TestRunInTxRollsBackFailedWork(t *testing.T) {
	wantErr := errors.New("write failed")
	tx := &fakeTx{}
	err := RunInTx(context.Background(), &fakeBeginner{tx: tx}, func(context.Context, pgx.Tx) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("RunInTx() error = %v, want %v", err, wantErr)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commits = %d, rollbacks = %d", tx.commits, tx.rollbacks)
	}
}

func TestRunInTxJoinsWorkAndRollbackErrors(t *testing.T) {
	workErr := errors.New("write failed")
	rollbackErr := errors.New("rollback failed")
	tx := &fakeTx{rollbackErr: rollbackErr}
	err := RunInTx(context.Background(), &fakeBeginner{tx: tx}, func(context.Context, pgx.Tx) error {
		return workErr
	})

	if !errors.Is(err, workErr) || !errors.Is(err, rollbackErr) {
		t.Fatalf("RunInTx() error = %v, want both callback and rollback errors", err)
	}
}

func TestRunInTxReturnsBeginAndCommitErrors(t *testing.T) {
	beginErr := errors.New("begin failed")
	if err := RunInTx(context.Background(), &fakeBeginner{beginErr: beginErr}, func(context.Context, pgx.Tx) error { return nil }); !errors.Is(err, beginErr) {
		t.Fatalf("begin error = %v, want %v", err, beginErr)
	}

	commitErr := errors.New("commit failed")
	tx := &fakeTx{commitErr: commitErr}
	if err := RunInTx(context.Background(), &fakeBeginner{tx: tx}, func(context.Context, pgx.Tx) error { return nil }); !errors.Is(err, commitErr) {
		t.Fatalf("commit error = %v, want %v", err, commitErr)
	}
}

func TestRunInTxRollsBackWhenWorkPanics(t *testing.T) {
	tx := &fakeTx{}
	func() {
		defer func() {
			if recovered := recover(); recovered != "boom" {
				t.Fatalf("recovered = %v, want boom", recovered)
			}
		}()
		_ = RunInTx(context.Background(), &fakeBeginner{tx: tx}, func(context.Context, pgx.Tx) error {
			panic("boom")
		})
	}()

	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commits = %d, rollbacks = %d", tx.commits, tx.rollbacks)
	}
}

func TestRunInTxRejectsNilDependencies(t *testing.T) {
	if err := RunInTx(context.Background(), nil, func(context.Context, pgx.Tx) error { return nil }); err == nil {
		t.Fatal("nil beginner must fail")
	}
	if err := RunInTx(context.Background(), &fakeBeginner{tx: &fakeTx{}}, nil); err == nil {
		t.Fatal("nil callback must fail")
	}
}

func TestRunInTxRejectsTypedNilBeginner(t *testing.T) {
	var beginner *fakeBeginner
	if err := RunInTx(context.Background(), beginner, func(context.Context, pgx.Tx) error { return nil }); err == nil {
		t.Fatal("typed nil beginner must fail")
	}
}
