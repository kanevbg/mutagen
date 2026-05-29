package forwarding

import (
	"context"
	"errors"
	"testing"
)

func TestHaltHonorsContextWhileWaitingForForwardingLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cancelled := false
	controller := &controller{
		session: &Session{
			Identifier: "forward_test",
		},
		cancel: func() {
			cancelled = true
		},
		done: make(chan struct{}),
	}

	err := controller.halt(ctx, controllerHaltModePause, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("halt did not report context cancellation: %v", err)
	}
	if !cancelled {
		t.Fatal("halt did not cancel forwarding loop")
	}
}
