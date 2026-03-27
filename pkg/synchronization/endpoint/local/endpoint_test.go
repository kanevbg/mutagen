package local

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mutagen-io/mutagen/pkg/state"
)

func TestShutdownPreemptsWorkersWaitingForScanLock(t *testing.T) {
	workerCtx, workerCancel := context.WithCancel(context.Background())
	saveCacheDone := make(chan struct{})
	watchDone := make(chan struct{})
	saveCacheSignal := make(chan struct{}, 1)
	scanLock := make(chan struct{}, 1)
	scanLock <- struct{}{}
	<-scanLock

	endpoint := &endpoint{
		workerCancel:  workerCancel,
		saveCacheDone: saveCacheDone,
		watchDone:     watchDone,
		pollSignal:    state.NewCoalescer(0),
		scanLock:      scanLock,
	}

	go func() {
		endpoint.saveCache(workerCtx, filepath.Join(t.TempDir(), "cache"), saveCacheSignal)
		close(saveCacheDone)
	}()
	saveCacheSignal <- struct{}{}

	go func() {
		endpoint.watchPoll(workerCtx, 3600, false)
		close(watchDone)
	}()

	shutdownResults := make(chan error, 1)
	go func() {
		shutdownResults <- endpoint.Shutdown()
	}()

	select {
	case err := <-shutdownResults:
		if err != nil {
			t.Fatal("shutdown failed:", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown hung while workers were waiting for the scan lock")
	}
}

func TestTransitionPreemptsWhileWaitingForScanLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scanLock := make(chan struct{}, 1)
	scanLock <- struct{}{}
	<-scanLock

	endpoint := &endpoint{
		scanLock: scanLock,
	}

	transitionResults := make(chan error, 1)
	go func() {
		_, _, _, err := endpoint.Transition(ctx, nil)
		transitionResults <- err
	}()

	select {
	case err := <-transitionResults:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("transition did not report cancellation:", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transition hung while waiting for the scan lock")
	}
}
