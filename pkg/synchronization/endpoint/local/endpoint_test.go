package local

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mutagen-io/mutagen/pkg/state"
	"github.com/mutagen-io/mutagen/pkg/synchronization/core"
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

func TestStagePreemptsWhileWaitingForScanLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scanLock := make(chan struct{}, 1)
	scanLock <- struct{}{}
	<-scanLock

	endpoint := &endpoint{
		scanLock: scanLock,
	}

	stageResults := make(chan error, 1)
	go func() {
		_, _, _, err := endpoint.Stage(ctx, []string{"file"}, [][]byte{{1}})
		stageResults <- err
	}()

	select {
	case err := <-stageResults:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("stage did not report cancellation:", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stage hung while waiting for the scan lock")
	}
}

func TestFilterUnsupportedTransitionsFiltersNestedUnsupportedContent(t *testing.T) {
	old := &core.Entry{
		Kind: core.EntryKind_Directory,
		Contents: map[string]*core.Entry{
			"kept.txt": {Kind: core.EntryKind_File, Digest: []byte{1}},
		},
	}
	new := &core.Entry{
		Kind: core.EntryKind_Directory,
		Contents: map[string]*core.Entry{
			"kept.txt":     {Kind: core.EntryKind_File, Digest: []byte{2}},
			"invalid/name": {Kind: core.EntryKind_File, Digest: []byte{3}},
		},
	}

	filtered, problems, err := (&endpoint{}).FilterUnsupportedTransitions([]*core.Change{{
		Path: "root",
		Old:  old,
		New:  new,
	}})
	if err != nil {
		t.Fatal("filter failed:", err)
	}
	if len(problems) != 1 {
		t.Fatalf("unexpected problem count: %d", len(problems))
	}
	if problems[0].Path != "root/invalid/name" {
		t.Error("unexpected problem path:", problems[0].Path)
	}
	if !strings.Contains(problems[0].Error, "path unsupported by target filesystem") {
		t.Error("unexpected problem error:", problems[0].Error)
	}
	if len(filtered) != 1 {
		t.Fatalf("unexpected filtered transition count: %d", len(filtered))
	}
	expected := &core.Entry{
		Kind: core.EntryKind_Directory,
		Contents: map[string]*core.Entry{
			"kept.txt": {Kind: core.EntryKind_File, Digest: []byte{2}},
		},
	}
	if !filtered[0].New.Equal(expected, true) {
		t.Error("unsupported content not filtered from transition")
	}
}
