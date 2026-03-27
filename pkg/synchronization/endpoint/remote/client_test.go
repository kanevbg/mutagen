package remote

import (
	"bufio"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/mutagen-io/mutagen/pkg/encoding"
	streampkg "github.com/mutagen-io/mutagen/pkg/stream"
	"github.com/mutagen-io/mutagen/pkg/synchronization/rsync"
)

func TestClientStageNoOpSendsCompletionMarker(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	defer clientConnection.Close()
	defer serverConnection.Close()

	clientWriter := bufio.NewWriter(clientConnection)
	client := &endpointClient{
		closer:  clientConnection,
		flusher: streampkg.NewMultiFlusher(clientWriter),
		encoder: encoding.NewProtobufEncoder(clientWriter),
		decoder: encoding.NewProtobufDecoder(bufio.NewReader(clientConnection)),
	}

	serverReader := bufio.NewReader(serverConnection)
	serverWriter := bufio.NewWriter(serverConnection)
	serverDecoder := encoding.NewProtobufDecoder(serverReader)
	serverEncoder := encoding.NewProtobufEncoder(serverWriter)
	serverFlusher := streampkg.NewMultiFlusher(serverWriter)

	serverResults := make(chan error, 1)
	go func() {
		request := &EndpointRequest{}
		if err := serverDecoder.Decode(request); err != nil {
			serverResults <- err
			return
		} else if request.Stage == nil {
			serverResults <- errors.New("stage request not received")
			return
		}

		if err := serverEncoder.Encode(&StageResponse{}); err != nil {
			serverResults <- err
			return
		} else if err = serverFlusher.Flush(); err != nil {
			serverResults <- err
			return
		}

		completion := &rsync.Transmission{}
		if err := serverDecoder.Decode(completion); err != nil {
			serverResults <- err
		} else if !isStageCompletionTransmission(completion) {
			serverResults <- errors.New("stage completion marker not received")
		} else {
			serverResults <- nil
		}
	}()

	paths, signatures, receiver, err := client.Stage(context.Background(), []string{"file"}, [][]byte{{1}})
	if err != nil {
		t.Fatal("stage failed:", err)
	} else if len(paths) != 0 || len(signatures) != 0 || receiver != nil {
		t.Fatal("unexpected staging result for no-op stage")
	}

	select {
	case err := <-serverResults:
		if err != nil {
			t.Fatal("server validation failed:", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not observe stage completion marker")
	}
}

func TestClientStageReceiverFinalizationSendsCompletionMarker(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	defer clientConnection.Close()
	defer serverConnection.Close()

	clientWriter := bufio.NewWriter(clientConnection)
	client := &endpointClient{
		closer:  clientConnection,
		flusher: streampkg.NewMultiFlusher(clientWriter),
		encoder: encoding.NewProtobufEncoder(clientWriter),
		decoder: encoding.NewProtobufDecoder(bufio.NewReader(clientConnection)),
	}

	serverReader := bufio.NewReader(serverConnection)
	serverWriter := bufio.NewWriter(serverConnection)
	serverDecoder := encoding.NewProtobufDecoder(serverReader)
	serverEncoder := encoding.NewProtobufEncoder(serverWriter)
	serverFlusher := streampkg.NewMultiFlusher(serverWriter)

	serverResults := make(chan error, 1)
	go func() {
		request := &EndpointRequest{}
		if err := serverDecoder.Decode(request); err != nil {
			serverResults <- err
			return
		} else if request.Stage == nil {
			serverResults <- errors.New("stage request not received")
			return
		}

		response := &StageResponse{
			Signatures: []*rsync.Signature{{}},
		}
		if err := serverEncoder.Encode(response); err != nil {
			serverResults <- err
			return
		} else if err = serverFlusher.Flush(); err != nil {
			serverResults <- err
			return
		}

		completion := &rsync.Transmission{}
		if err := serverDecoder.Decode(completion); err != nil {
			serverResults <- err
		} else if !isStageCompletionTransmission(completion) {
			serverResults <- errors.New("stage completion marker not received")
		} else {
			serverResults <- nil
		}
	}()

	paths, signatures, receiver, err := client.Stage(context.Background(), []string{"file"}, [][]byte{{1}})
	if err != nil {
		t.Fatal("stage failed:", err)
	} else if len(paths) != 1 || len(signatures) != 1 || receiver == nil {
		t.Fatal("unexpected staging result for finalizable stage")
	}

	if err := rsync.FinalizeReceiver(receiver); err != nil {
		t.Fatal("unable to finalize stage receiver:", err)
	}

	select {
	case err := <-serverResults:
		if err != nil {
			t.Fatal("server validation failed:", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not observe stage completion marker after receiver finalization")
	}
}
