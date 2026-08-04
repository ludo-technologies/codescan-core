package lsh

import (
	"fmt"
	"slices"
)

const (
	defaultBands = 32
	defaultRows  = 4
)

// bandKey identifies one band's bucket. The band index and the hash of the
// band's signature slice together determine the bucket, so a comparable struct
// serves as the map key directly — no formatted string per band per fragment.
type bandKey struct {
	band int
	hash uint64
}

// LSHIndex implements MinHash LSH with banding.
type LSHIndex struct {
	bands      int
	rows       int
	buckets    map[bandKey][]string
	signatures map[string]*MinHashSignature
	// insertKeys is scratch space reused by AddFragment. Queries use their own
	// buffer so that reading the index stays free of mutation.
	insertKeys []bandKey
}

// NewLSHIndex creates an index with banding parameters.
func NewLSHIndex(bands, rows int) *LSHIndex {
	if bands <= 0 {
		bands = defaultBands
	}
	if rows <= 0 {
		rows = defaultRows
	}
	return &LSHIndex{
		bands:      bands,
		rows:       rows,
		buckets:    make(map[bandKey][]string),
		signatures: make(map[string]*MinHashSignature),
	}
}

// AddFragment inserts a fragment signature into the index.
func (idx *LSHIndex) AddFragment(id string, signature *MinHashSignature) error {
	if signature == nil || len(signature.signatures) == 0 {
		return fmt.Errorf("empty signature for id %s", id)
	}
	if id == "" {
		return fmt.Errorf("empty fragment id")
	}
	_, reindexed := idx.signatures[id]
	idx.signatures[id] = signature
	idx.addToBuckets(id, signature, reindexed)
	return nil
}

// BuildIndex is a no-op for incremental building (kept for API symmetry).
func (idx *LSHIndex) BuildIndex() error { return nil }

// FindCandidates retrieves candidate fragment IDs that share at least one band bucket.
func (idx *LSHIndex) FindCandidates(signature *MinHashSignature) []string {
	return idx.FindCandidatesLimit(signature, 0)
}

// FindCandidatesLimit retrieves candidate fragment IDs that share at least one
// band bucket, stopping as soon as maxCandidates distinct IDs have been
// collected so dense buckets never materialize in full. maxCandidates <= 0
// disables the cap. Traversal order is deterministic — band order, then bucket
// insertion order — so capped queries keep the earliest-encountered
// candidates rather than an arbitrary subset.
func (idx *LSHIndex) FindCandidatesLimit(signature *MinHashSignature, maxCandidates int) []string {
	if signature == nil || len(signature.signatures) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{})
	out := []string{}
	for _, key := range idx.appendBandKeys(nil, signature) {
		for _, id := range idx.buckets[key] {
			if maxCandidates > 0 && len(out) >= maxCandidates {
				return out
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// GetSignature returns the stored signature for a fragment ID.
func (idx *LSHIndex) GetSignature(id string) *MinHashSignature {
	return idx.signatures[id]
}

// Size returns the number of fragments in the index.
func (idx *LSHIndex) Size() int {
	return len(idx.signatures)
}

// Bands returns the number of bands.
func (idx *LSHIndex) Bands() int {
	return idx.bands
}

// Rows returns the number of rows per band.
func (idx *LSHIndex) Rows() int {
	return idx.rows
}

// addToBuckets files a fragment under each of its band keys. A fragment being
// indexed for the first time cannot already be in any bucket, so the duplicate
// scan — linear in bucket size, and buckets grow large when many fragments hash
// alike — only runs when the same ID is indexed again.
func (idx *LSHIndex) addToBuckets(id string, sig *MinHashSignature, reindexed bool) {
	idx.insertKeys = idx.appendBandKeys(idx.insertKeys[:0], sig)
	for _, k := range idx.insertKeys {
		cur := idx.buckets[k]
		if reindexed && slices.Contains(cur, id) {
			continue
		}
		idx.buckets[k] = append(cur, id)
	}
}

// appendBandKeys appends the band keys of a signature to dst and returns the
// extended slice, so callers that run repeatedly can reuse one buffer.
func (idx *LSHIndex) appendBandKeys(dst []bandKey, sig *MinHashSignature) []bandKey {
	total := len(sig.signatures)
	r := idx.rows
	b := idx.bands
	if r <= 0 {
		r = defaultRows
	}
	if b <= 0 {
		b = defaultBands
	}
	maxBands := total / r
	if b > maxBands {
		b = maxBands
	}
	if cap(dst) < len(dst)+b {
		grown := make([]bandKey, len(dst), len(dst)+b)
		copy(grown, dst)
		dst = grown
	}

	for band := 0; band < b; band++ {
		start := band * r
		end := start + r
		if end > total {
			end = total
		}

		// FNV-1a over the big-endian bytes of the band's signature values.
		h := uint64(fnvOffsetBasis64)
		for _, v := range sig.signatures[start:end] {
			for shift := 56; shift >= 0; shift -= 8 {
				h ^= (v >> uint(shift)) & 0xff
				h *= fnvPrime64
			}
		}
		dst = append(dst, bandKey{band: band, hash: h})
	}

	return dst
}
