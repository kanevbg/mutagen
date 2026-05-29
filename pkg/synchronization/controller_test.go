package synchronization

import (
	"context"
	"errors"
	"testing"
)

func TestHaltHonorsContextWhileWaitingForSynchronizationLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cancelled := false
	controller := &controller{
		session: &Session{
			Identifier: "sync_test",
		},
		cancel: func() {
			cancelled = true
		},
		done: make(chan struct{}),
	}

	err := controller.halt(ctx, controllerHaltModePause, "", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("halt did not report context cancellation: %v", err)
	}
	if !cancelled {
		t.Fatal("halt did not cancel synchronization loop")
	}
}
