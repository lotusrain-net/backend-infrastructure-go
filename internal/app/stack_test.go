package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type closeFunc func(context.Context) error

func (fn closeFunc) Close(ctx context.Context) error { return fn(ctx) }

func TestStackClosesResourcesInReverseConstructionOrder(t *testing.T) {
	var order []string
	stack := NewStack()
	stack.Add(closeFunc(func(context.Context) error { order = append(order, "database"); return nil }))
	stack.Add(closeFunc(func(context.Context) error { order = append(order, "redis"); return nil }))
	stack.Add(closeFunc(func(context.Context) error { order = append(order, "server"); return nil }))
	if err := stack.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"server", "redis", "database"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order=%v want=%v", order, want)
	}
}

func TestStackJoinsCloseErrorsAndContinues(t *testing.T) {
	wantA, wantB := errors.New("a"), errors.New("b")
	stack := NewStack()
	stack.Add(closeFunc(func(context.Context) error { return wantA }))
	stack.Add(closeFunc(func(context.Context) error { return wantB }))
	err := stack.Close(context.Background())
	if !errors.Is(err, wantA) || !errors.Is(err, wantB) {
		t.Fatalf("error=%v", err)
	}
}

func TestBuildResourcesClosesConstructedResourcesWhenDependencyFails(t *testing.T) {
	want := errors.New("redis unavailable")
	closed := false
	_, err := BuildResources(context.Background(),
		func(context.Context) (Closer, error) {
			return closeFunc(func(context.Context) error { closed = true; return nil }), nil
		},
		func(context.Context) (Closer, error) { return nil, want },
	)
	if !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
	if !closed {
		t.Fatal("constructed dependency was not closed")
	}
}

func TestRoleRuntimesCloseTheirResourceStacks(t *testing.T) {
	tests := []struct {
		name     string
		shutdown func(*Stack) error
	}{
		{name: "api", shutdown: func(stack *Stack) error { return (&apiRuntime{resources: stack}).Shutdown(context.Background()) }},
		{name: "worker", shutdown: func(stack *Stack) error { return (&workerRuntime{resources: stack}).Shutdown(context.Background()) }},
		{name: "scheduler", shutdown: func(stack *Stack) error { return (&schedulerRuntime{resources: stack}).Shutdown(context.Background()) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			closed := false
			stack := NewStack()
			stack.Add(closeFunc(func(context.Context) error {
				closed = true
				return nil
			}))
			if err := test.shutdown(stack); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
			if !closed {
				t.Fatal("resource stack was not closed")
			}
		})
	}
}
