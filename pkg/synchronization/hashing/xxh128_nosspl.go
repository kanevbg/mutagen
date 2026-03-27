//go:build !mutagensspl

package hashing

import (
	"hash"

	"github.com/zeebo/xxh3"
)

// xxh128SupportStatus returns XXH128 hashing support status.
func xxh128SupportStatus() AlgorithmSupportStatus {
	return AlgorithmSupportStatusSupported
}

// newXXH128Factory creates a new hasher factory for XXH128 hashers.
func newXXH128Factory() func() hash.Hash {
	return func() hash.Hash {
		return &xxh128Hash{xxh3.New()}
	}
}

// xxh128Hash implements hash.Hash using the XXH128 algorithm.
type xxh128Hash struct {
	// Hasher is the underlying hasher.
	*xxh3.Hasher
}

// Sum implements hash.Hash.Sum.
func (h *xxh128Hash) Sum(b []byte) []byte {
	// Compute the sum and associated bytes.
	sum128 := h.Sum128()
	sum128Bytes := sum128.Bytes()

	// If b is nil, then take the fast way out.
	if b == nil {
		return sum128Bytes[:]
	}

	// Otherwise append the bytes to b.
	return append(b, sum128Bytes[:]...)
}
