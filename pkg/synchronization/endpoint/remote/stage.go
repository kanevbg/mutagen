package remote

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mutagen-io/mutagen/pkg/encoding"
	streampkg "github.com/mutagen-io/mutagen/pkg/stream"
	"github.com/mutagen-io/mutagen/pkg/synchronization/rsync"
)

const (
	// stageCompletionExpectedSize is the expected size value used in the stage
	// completion marker. The marker is intentionally encoded as an invalid rsync
	// transmission so that it can share the same stream without ambiguity.
	stageCompletionExpectedSize = ^uint64(0)
	// stageCompletionError is the error string used in the stage completion
	// marker.
	stageCompletionError = "stage completion"
)

// newStageCompletionTransmission creates the marker used to delimit stage
// completion on the rsync transmission stream.
func newStageCompletionTransmission() *rsync.Transmission {
	return &rsync.Transmission{
		ExpectedSize: stageCompletionExpectedSize,
		Done:         true,
		Error:        stageCompletionError,
	}
}

// isStageCompletionTransmission returns whether or not the specified
// transmission is the special stage completion marker.
func isStageCompletionTransmission(transmission *rsync.Transmission) bool {
	return transmission != nil &&
		transmission.ExpectedSize == stageCompletionExpectedSize &&
		transmission.Done &&
		transmission.Error == stageCompletionError &&
		transmission.Operation == nil
}

// stageCompletionSignaler transmits the stage completion marker at most once.
type stageCompletionSignaler struct {
	encoder *encoding.ProtobufEncoder
	flusher streampkg.Flusher
	once    sync.Once
	done    chan struct{}
	err     error
}

// newStageCompletionSignaler creates a new stage completion signaler.
func newStageCompletionSignaler(
	encoder *encoding.ProtobufEncoder,
	flusher streampkg.Flusher,
) *stageCompletionSignaler {
	return &stageCompletionSignaler{
		encoder: encoder,
		flusher: flusher,
		done:    make(chan struct{}),
	}
}

// signal transmits the stage completion marker if it hasn't already been sent.
func (s *stageCompletionSignaler) signal() error {
	s.once.Do(func() {
		if err := s.encoder.Encode(newStageCompletionTransmission()); err != nil {
			s.err = fmt.Errorf("unable to encode stage completion marker: %w", err)
		} else if err = s.flusher.Flush(); err != nil {
			s.err = fmt.Errorf("unable to transmit stage completion marker: %w", err)
		}
		close(s.done)
	})
	<-s.done
	return s.err
}

// stageCompletionEncoder wraps the rsync encoder used for staged data transfer
// and emits the stage completion marker when the transfer is finalized.
type stageCompletionEncoder struct {
	encoder  *protobufRsyncEncoder
	signaler *stageCompletionSignaler
}

// Encode implements rsync.Encoder.Encode.
func (e *stageCompletionEncoder) Encode(transmission *rsync.Transmission) error {
	return e.encoder.Encode(transmission)
}

// Finalize implements rsync.Encoder.Finalize.
func (e *stageCompletionEncoder) Finalize() error {
	if err := e.encoder.Finalize(); err != nil {
		return err
	}
	return e.signaler.signal()
}

// stageTransmissionResult stores asynchronous stage message decoding results.
type stageTransmissionResult struct {
	transmission *rsync.Transmission
	err          error
}

// receiveStageTransmissionAsync starts decoding the next inbound stage message.
func receiveStageTransmissionAsync(decoder *encoding.ProtobufDecoder) <-chan stageTransmissionResult {
	results := make(chan stageTransmissionResult, 1)
	go func() {
		transmission := &rsync.Transmission{}
		if err := decoder.Decode(transmission); err != nil {
			results <- stageTransmissionResult{
				err: fmt.Errorf("unable to receive stage message: %w", err),
			}
		} else {
			results <- stageTransmissionResult{transmission: transmission}
		}
	}()
	return results
}

// receiveStageCompletion waits for and validates the stage completion marker.
func receiveStageCompletion(next <-chan stageTransmissionResult) error {
	result := <-next
	if result.err != nil {
		return result.err
	} else if !isStageCompletionTransmission(result.transmission) {
		return errors.New("unexpected stage message received while awaiting completion")
	}
	return nil
}

// decodeStageOperations forwards stage transmissions to a receiver until all
// file streams are complete or a stage completion marker is received. It
// returns the next pending stage message future when all file streams have been
// successfully decoded.
func decodeStageOperations(
	next <-chan stageTransmissionResult,
	decoder *encoding.ProtobufDecoder,
	count uint64,
	receiver rsync.Receiver,
) (<-chan stageTransmissionResult, bool, error) {
	for count > 0 {
		result := <-next
		if result.err != nil {
			rsync.FinalizeReceiver(receiver)
			return nil, false, result.err
		}

		transmission := result.transmission
		if isStageCompletionTransmission(transmission) {
			rsync.FinalizeReceiver(receiver)
			return nil, true, nil
		} else if err := transmission.EnsureValid(); err != nil {
			rsync.FinalizeReceiver(receiver)
			return nil, false, fmt.Errorf("invalid transmission received: %w", err)
		} else if err = receiver.Receive(transmission); err != nil {
			rsync.FinalizeReceiver(receiver)
			return nil, false, fmt.Errorf("unable to forward message to receiver: %w", err)
		}

		if transmission.Done {
			count--
		}
		if count > 0 {
			next = receiveStageTransmissionAsync(decoder)
		}
	}

	if err := rsync.FinalizeReceiver(receiver); err != nil {
		return nil, false, fmt.Errorf("unable to finalize stage receiver: %w", err)
	}

	return receiveStageTransmissionAsync(decoder), false, nil
}
