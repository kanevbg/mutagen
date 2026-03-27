package compression

import (
	"bytes"
	"io"
	"testing"
)

// TestAlgorithmUnmarshal tests that unmarshaling from a string specification
// succeeeds for Algorithm.
func TestAlgorithmUnmarshal(t *testing.T) {
	// Set up test cases.
	testCases := []struct {
		text          string
		expected      Algorithm
		expectFailure bool
	}{
		{"", Algorithm_AlgorithmDefault, true},
		{"asdf", Algorithm_AlgorithmDefault, true},
		{"none", Algorithm_AlgorithmNone, false},
		{"deflate", Algorithm_AlgorithmDeflate, false},
		{"zstandard", Algorithm_AlgorithmZstandard, false},
	}

	// Process test cases.
	for _, testCase := range testCases {
		var algorithm Algorithm
		if err := algorithm.UnmarshalText([]byte(testCase.text)); err != nil {
			if !testCase.expectFailure {
				t.Errorf("unable to unmarshal text (%s): %s", testCase.text, err)
			}
		} else if testCase.expectFailure {
			t.Error("unmarshaling succeeded unexpectedly for text:", testCase.text)
		} else if algorithm != testCase.expected {
			t.Errorf(
				"unmarshaled algorithm (%s) does not match expected (%s)",
				algorithm,
				testCase.expected,
			)
		}
	}
}

// TestAlgorithmSupportStatus tests that Algorithm support detection works as
// expected.
func TestAlgorithmSupportStatus(t *testing.T) {
	// Set up test cases.
	testCases := []struct {
		algorithm Algorithm
		expected  AlgorithmSupportStatus
	}{
		{Algorithm_AlgorithmDefault, AlgorithmSupportStatusUnsupported},
		{Algorithm_AlgorithmNone, AlgorithmSupportStatusSupported},
		{Algorithm_AlgorithmDeflate, AlgorithmSupportStatusSupported},
		{Algorithm_AlgorithmZstandard, zstandardSupportStatus()},
		{(Algorithm_AlgorithmZstandard + 1), AlgorithmSupportStatusUnsupported},
	}

	// Process test cases.
	for _, testCase := range testCases {
		if supportStatus := testCase.algorithm.SupportStatus(); supportStatus != testCase.expected {
			t.Errorf(
				"algorithm support status (%d) does not match expected (%d)",
				supportStatus,
				testCase.expected,
			)
		}
	}
}

// TestAlgorithmDescription tests that Algorithm description generation works as
// expected.
func TestAlgorithmDescription(t *testing.T) {
	// Set up test cases.
	testCases := []struct {
		algorithm Algorithm
		expected  string
	}{
		{Algorithm_AlgorithmDefault, "Default"},
		{Algorithm_AlgorithmNone, "None"},
		{Algorithm_AlgorithmDeflate, "DEFLATE"},
		{Algorithm_AlgorithmZstandard, "Zstandard"},
		{(Algorithm_AlgorithmZstandard + 1), "Unknown"},
	}

	// Process test cases.
	for _, testCase := range testCases {
		if description := testCase.algorithm.Description(); description != testCase.expected {
			t.Errorf(
				"algorithm description (%s) does not match expected (%s)",
				description,
				testCase.expected,
			)
		}
	}
}

// TestAlgorithmRoundTrip verifies compression/decompression round-tripping for
// supported algorithms.
func TestAlgorithmRoundTrip(t *testing.T) {
	testCases := []Algorithm{
		Algorithm_AlgorithmNone,
		Algorithm_AlgorithmDeflate,
		Algorithm_AlgorithmZstandard,
	}

	payload := []byte("mutagen-compression-round-trip-payload")

	for _, algorithm := range testCases {
		if algorithm.SupportStatus() != AlgorithmSupportStatusSupported {
			continue
		}

		var compressed bytes.Buffer
		compressor := algorithm.Compress(&compressed)
		if _, err := compressor.Write(payload); err != nil {
			t.Fatalf("unable to compress payload with %s: %v", algorithm, err)
		}
		if err := compressor.Close(); err != nil {
			t.Fatalf("unable to close compressor for %s: %v", algorithm, err)
		}

		decompressor := algorithm.Decompress(&compressed)
		result, err := io.ReadAll(decompressor)
		if err != nil {
			t.Fatalf("unable to decompress payload with %s: %v", algorithm, err)
		}
		if err := decompressor.Close(); err != nil {
			t.Fatalf("unable to close decompressor for %s: %v", algorithm, err)
		}

		if !bytes.Equal(result, payload) {
			t.Fatalf("decompressed data mismatch for %s", algorithm)
		}
	}
}
