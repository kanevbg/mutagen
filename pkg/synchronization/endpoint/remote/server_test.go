package remote

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/mutagen-io/mutagen/pkg/encoding"
	streampkg "github.com/mutagen-io/mutagen/pkg/stream"
	"github.com/mutagen-io/mutagen/pkg/synchronization/core"
	"github.com/mutagen-io/mutagen/pkg/synchronization/rsync"
)

type testStageEndpoint struct {
	stage func(context.Context, []string, [][]byte) ([]string, []*rsync.Signature, rsync.Receiver, error)
}

func (e *testStageEndpoint) Poll(context.Context) error { panic("unexpected Poll call") }

func (e *testStageEndpoint) Scan(context.Context, *core.Entry, bool) (*core.Snapshot, error, bool) {
	panic("unexpected Scan call")
}

func (e *testStageEndpoint) Stage(
	ctx context.Context,
	paths []string,
	digests [][]byte,
) ([]string, []*rsync.Signature, rsync.Receiver, error) {
	return e.stage(ctx, paths, digests)
}

func (e *testStageEndpoint) Supply([]string, []*rsync.Signature, rsync.Receiver) error {
	panic("unexpected Supply call")
}

func (e *testStageEndpoint) FilterUnsupportedTransitions(transitions []*core.Change) ([]*core.Change, []*core.Problem, error) {
	return transitions, nil, nil
}

func (e *testStageEndpoint) Transition(context.Context, []*core.Change) ([]*core.Entry, []*core.Problem, bool, error) {
	panic("unexpected Transition call")
}

func (e *testStageEndpoint) Shutdown() error { return nil }

func TestServeStageCancelsInitializationOnTransportClosure(t *testing.T) {
	client, serverConnection := net.Pipe()
	defer client.Close()
	defer serverConnection.Close()

	serverWriter := bufio.NewWriter(serverConnection)
	server := &endpointServer{
		endpoint: &testStageEndpoint{
			stage: func(ctx context.Context, _ []string, _ [][]byte) ([]string, []*rsync.Signature, rsync.Receiver, error) {
				<-ctx.Done()
				return nil, nil, nil, ctx.Err()
			},
		},
		flusher: streampkg.NewMultiFlusher(serverWriter),
		encoder: encoding.NewProtobufEncoder(serverWriter),
		decoder: encoding.NewProtobufDecoder(bufio.NewReader(serverConnection)),
	}

	results := make(chan error, 1)
	go func() {
		results <- server.serveStage(&StageRequest{
			Paths:   []string{"file"},
			Digests: [][]byte{{1}},
		})
	}()

	if err := client.Close(); err != nil {
		t.Fatal("unable to close client connection:", err)
	}

	select {
	case err := <-results:
		if err == nil {
			t.Fatal("expected stage serving error on transport closure")
		}
	case <-time.After(time.Second):
		t.Fatal("stage serving did not unblock on transport closure")
	}
}

func TestServeStageWaitsForCompletionMarkerOnNoOp(t *testing.T) {
	client, serverConnection := net.Pipe()
	defer client.Close()
	defer serverConnection.Close()

	clientReader := bufio.NewReader(client)
	clientWriter := bufio.NewWriter(client)
	clientEncoder := encoding.NewProtobufEncoder(clientWriter)
	clientDecoder := encoding.NewProtobufDecoder(clientReader)
	clientFlusher := streampkg.NewMultiFlusher(clientWriter)

	serverWriter := bufio.NewWriter(serverConnection)
	server := &endpointServer{
		endpoint: &testStageEndpoint{
			stage: func(context.Context, []string, [][]byte) ([]string, []*rsync.Signature, rsync.Receiver, error) {
				return nil, nil, nil, nil
			},
		},
		flusher: streampkg.NewMultiFlusher(serverWriter),
		encoder: encoding.NewProtobufEncoder(serverWriter),
		decoder: encoding.NewProtobufDecoder(bufio.NewReader(serverConnection)),
	}

	results := make(chan error, 1)
	go func() {
		results <- server.serveStage(&StageRequest{
			Paths:   []string{"file"},
			Digests: [][]byte{{1}},
		})
	}()

	response := &StageResponse{}
	if err := clientDecoder.Decode(response); err != nil {
		t.Fatal("unable to receive stage response:", err)
	} else if err = response.ensureValid([]string{"file"}); err != nil {
		t.Fatal("invalid stage response received:", err)
	}

	select {
	case err := <-results:
		t.Fatal("stage serving returned before completion marker:", err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := clientEncoder.Encode(newStageCompletionTransmission()); err != nil {
		t.Fatal("unable to encode stage completion marker:", err)
	} else if err = clientFlusher.Flush(); err != nil {
		t.Fatal("unable to send stage completion marker:", err)
	}

	select {
	case err := <-results:
		if err != nil {
			t.Fatal("stage serving failed after completion marker:", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stage serving did not complete after completion marker")
	}
}
