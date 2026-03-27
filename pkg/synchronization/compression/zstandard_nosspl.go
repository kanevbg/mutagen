//go:build !mutagensspl

package compression

import (
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/mutagen-io/mutagen/pkg/stream"
)

// zstandardSupportStatus returns Zstandard compression support status.
func zstandardSupportStatus() AlgorithmSupportStatus {
	return AlgorithmSupportStatusSupported
}

// compressZstandard implements compression for Zstandard streams.
func compressZstandard(compressed io.Writer) stream.WriteFlushCloser {
	// We only use default configuration options, so construction errors would
	// indicate an internal inconsistency and are treated as unrecoverable.
	compressor, err := zstd.NewWriter(compressed)
	if err != nil {
		panic("Zstandard compressor construction failed")
	}
	return compressor
}

// decompressZstandard implements decompression for Zstandard streams.
func decompressZstandard(compressed io.Reader) io.ReadCloser {
	// We only use default configuration options, so construction errors would
	// indicate an internal inconsistency and are treated as unrecoverable.
	decompressor, err := zstd.NewReader(compressed)
	if err != nil {
		panic("Zstandard decompressor construction failed")
	}
	return decompressor.IOReadCloser()
}
