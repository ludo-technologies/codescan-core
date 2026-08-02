package lsh

import (
	"fmt"
	"strconv"
	"testing"
)

// benchFeatures builds a deterministic feature set for one fragment. Fragments
// share a common vocabulary so signatures collide the way real near-duplicate
// code fragments do.
func benchFeatures(fragment, count int) []string {
	features := make([]string, count)
	for i := range features {
		features[i] = "feature-" + strconv.Itoa((fragment+i*7)%(count*4))
	}
	return features
}

func BenchmarkComputeSignature_BySetSize(b *testing.B) {
	for _, size := range []int{16, 64, 256, 1024} {
		b.Run(fmt.Sprintf("features=%d", size), func(b *testing.B) {
			hasher := NewMinHasher(defaultNumHashes)
			features := benchFeatures(0, size)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hasher.ComputeSignature(features)
			}
		})
	}
}

// BenchmarkLSHIndex_Build measures index construction at fragment counts a
// large monorepo reaches, where per-insert costs that look negligible at small
// scale dominate.
func BenchmarkLSHIndex_Build(b *testing.B) {
	for _, fragments := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("fragments=%d", fragments), func(b *testing.B) {
			hasher := NewMinHasher(defaultNumHashes)
			signatures := make([]*MinHashSignature, fragments)
			ids := make([]string, fragments)
			for i := range signatures {
				signatures[i] = hasher.ComputeSignature(benchFeatures(i, 32))
				ids[i] = strconv.Itoa(i)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				index := NewLSHIndex(defaultBands, defaultRows)
				for j, signature := range signatures {
					if err := index.AddFragment(ids[j], signature); err != nil {
						b.Fatalf("AddFragment: %v", err)
					}
				}
			}
		})
	}
}

// BenchmarkLSHIndex_DenseBuckets is the pathological case for clone detection:
// many fragments whose signatures are identical, so they all land in the same
// buckets.
func BenchmarkLSHIndex_DenseBuckets(b *testing.B) {
	const fragments = 20_000

	hasher := NewMinHasher(defaultNumHashes)
	signature := hasher.ComputeSignature(benchFeatures(0, 32))
	ids := make([]string, fragments)
	for i := range ids {
		ids[i] = strconv.Itoa(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index := NewLSHIndex(defaultBands, defaultRows)
		for _, id := range ids {
			if err := index.AddFragment(id, signature); err != nil {
				b.Fatalf("AddFragment: %v", err)
			}
		}
	}
}

func BenchmarkLSHIndex_FindCandidates(b *testing.B) {
	const fragments = 50_000
	const maxCandidates = 1024

	hasher := NewMinHasher(defaultNumHashes)
	index := NewLSHIndex(defaultBands, defaultRows)
	queries := make([]*MinHashSignature, 0, 64)
	for i := 0; i < fragments; i++ {
		signature := hasher.ComputeSignature(benchFeatures(i, 32))
		if err := index.AddFragment(strconv.Itoa(i), signature); err != nil {
			b.Fatalf("AddFragment: %v", err)
		}
		if len(queries) < cap(queries) {
			queries = append(queries, signature)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.FindCandidatesLimit(queries[i%len(queries)], maxCandidates)
	}
}
