package lsh

import (
	"math"
	"math/rand"
)

const (
	defaultNumHashes = 128
	hashSeed         = 0x5eed_1234_cafe_babe
)

// MinHashSignature holds the signature vector.
type MinHashSignature struct {
	signatures []uint64
	numHashes  int
}

// Signatures returns the signature slice.
func (s *MinHashSignature) Signatures() []uint64 {
	return s.signatures
}

// NumHashes returns the number of hash functions used.
func (s *MinHashSignature) NumHashes() int {
	return s.numHashes
}

// MinHasher computes MinHash signatures for feature sets.
//
// The hash family is stored as coefficient pairs rather than closures: signing
// one fragment evaluates numHashes × |features| hashes, so an indirect call per
// evaluation is the difference between a few and a few dozen instructions.
type MinHasher struct {
	numHashes int
	factors   []uint64
	offsets   []uint64
}

// NewMinHasher creates a MinHasher with numHashes functions (default 128 if invalid).
func NewMinHasher(numHashes int) *MinHasher {
	if numHashes <= 0 {
		numHashes = defaultNumHashes
	}
	mh := &MinHasher{numHashes: numHashes}
	mh.generateHashFunctions()
	return mh
}

func (m *MinHasher) generateHashFunctions() {
	rng := rand.New(rand.NewSource(hashSeed))
	m.factors = make([]uint64, m.numHashes)
	m.offsets = make([]uint64, m.numHashes)
	for i := 0; i < m.numHashes; i++ {
		m.factors[i] = rng.Uint64() | 1 // odd to avoid trivial cycles
		m.offsets[i] = rng.Uint64()
	}
}

// ComputeSignature computes the MinHash signature for a set of features.
func (m *MinHasher) ComputeSignature(features []string) *MinHashSignature {
	sig := make([]uint64, m.numHashes)
	if len(features) == 0 {
		return &MinHashSignature{signatures: sig, numHashes: m.numHashes}
	}

	set := make(map[string]struct{}, len(features))
	base := make([]uint64, 0, len(features))
	for _, f := range features {
		if _, duplicate := set[f]; duplicate {
			continue
		}
		set[f] = struct{}{}
		base = append(base, Hash64(f))
	}

	// Hash-outer, feature-inner: the running minimum stays in a register for the
	// whole inner loop, and the constant addend is computed once per hash rather
	// than once per feature.
	for i, factor := range m.factors {
		offset := m.offsets[i]
		addend := factor + offset
		minimum := uint64(math.MaxUint64)
		for _, x := range base {
			if v := ((factor * x) ^ offset) + addend; v < minimum {
				minimum = v
			}
		}
		sig[i] = minimum
	}

	return &MinHashSignature{signatures: sig, numHashes: m.numHashes}
}

// EstimateJaccardSimilarity estimates Jaccard similarity via signature agreement ratio.
func (m *MinHasher) EstimateJaccardSimilarity(sig1, sig2 *MinHashSignature) float64 {
	if sig1 == nil || sig2 == nil || len(sig1.signatures) == 0 || len(sig2.signatures) == 0 {
		return 0.0
	}
	n := MinInt(len(sig1.signatures), len(sig2.signatures))
	if n == 0 {
		return 0.0
	}
	match := 0
	for i := 0; i < n; i++ {
		if sig1.signatures[i] == sig2.signatures[i] {
			match++
		}
	}
	return float64(match) / float64(n)
}

// NumHashes returns the number of hash functions.
func (m *MinHasher) NumHashes() int { return m.numHashes }

// FNV-1a 64-bit parameters. Hashing is open-coded rather than routed through
// hash/fnv so that hashing a string does not allocate a byte slice and a hasher
// on every call.
const (
	fnvOffsetBasis64 = 14695981039346656037
	fnvPrime64       = 1099511628211
)

// Hash64 computes a 64-bit FNV-1a hash for a string.
func Hash64(s string) uint64 {
	h := uint64(fnvOffsetBasis64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}

// MinInt returns the smaller of two ints.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
