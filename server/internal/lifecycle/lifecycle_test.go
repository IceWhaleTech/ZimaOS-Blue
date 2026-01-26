package lifecycle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}
	if m.Context() == nil {
		t.Error("Context() returned nil")
	}
}

func TestManager_Go(t *testing.T) {
	m := New()

	var executed atomic.Bool
	m.Go(func(ctx context.Context) {
		executed.Store(true)
	})

	m.Wait()

	if !executed.Load() {
		t.Error("Goroutine was not executed")
	}
}

func TestManager_Shutdown(t *testing.T) {
	m := New()

	var counter atomic.Int32
	m.Go(func(ctx context.Context) {
		<-ctx.Done()
		counter.Add(1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	if counter.Load() != 1 {
		t.Errorf("Counter = %v, want 1", counter.Load())
	}
}

func TestManager_ShutdownHooks(t *testing.T) {
	m := New()

	var order []int
	m.RegisterShutdownHook(func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})
	m.RegisterShutdownHook(func(ctx context.Context) error {
		order = append(order, 2)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	// Hooks should be called in reverse order
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Errorf("Hooks order = %v, want [2, 1]", order)
	}
}

func TestManager_Done(t *testing.T) {
	m := New()

	select {
	case <-m.Done():
		t.Error("Done() should not be closed before shutdown")
	default:
		// Expected
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.Shutdown(ctx)

	select {
	case <-m.Done():
		// Expected
	case <-time.After(time.Second):
		t.Error("Done() should be closed after shutdown")
	}
}

func TestManager_ShutdownTimeout(t *testing.T) {
	m := New()

	// Start a goroutine that never finishes
	m.Go(func(ctx context.Context) {
		<-ctx.Done()
		time.Sleep(10 * time.Second) // Simulate slow cleanup
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := m.Shutdown(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Shutdown() error = %v, want %v", err, context.DeadlineExceeded)
	}
}
