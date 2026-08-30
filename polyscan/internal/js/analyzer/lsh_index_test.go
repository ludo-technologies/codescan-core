package analyzer

import (
	"reflect"
	"testing"

	corelsh "github.com/ludo-technologies/polyscan/core/lsh"
)

func TestLSHCandidateIndexConvertsSortsAndCapsIDs(t *testing.T) {
	mh := corelsh.NewMinHasher(128)
	sig := mh.ComputeSignature([]string{"same", "feature", "set"})

	lsh := newLSHCandidateIndex(32, 4, 3)
	for _, id := range []int{4, 2, 3, 1} {
		if err := lsh.AddFragment(id, sig); err != nil {
			t.Fatalf("add %d: %v", id, err)
		}
	}

	// The cap keeps the first candidates in traversal order ({4, 2, 3}),
	// not the smallest IDs; the capped set is then sorted ascending.
	want := []int{2, 3, 4}
	for i := 0; i < 10; i++ {
		if got := lsh.FindCandidates(sig); !reflect.DeepEqual(got, want) {
			t.Fatalf("candidate mismatch: got %v want %v", got, want)
		}
	}
}

func TestLSHCandidateIndexUsesDefaultCap(t *testing.T) {
	mh := corelsh.NewMinHasher(128)
	sig := mh.ComputeSignature([]string{"same", "feature", "set"})
	lsh := newLSHCandidateIndex(32, 4, 0)
	for id := defaultLSHMaxCandidates; id >= 0; id-- {
		if err := lsh.AddFragment(id, sig); err != nil {
			t.Fatalf("add %d: %v", id, err)
		}
	}

	// Descending insertion: the default cap keeps the first-inserted
	// defaultLSHMaxCandidates candidates (IDs 1024..1), sorted ascending.
	got := lsh.FindCandidates(sig)
	if len(got) != defaultLSHMaxCandidates || got[0] != 1 || got[len(got)-1] != defaultLSHMaxCandidates {
		t.Fatalf("default cap or order mismatch: len=%d first=%d last=%d", len(got), got[0], got[len(got)-1])
	}
}

func TestLSHCandidateIndexRejectsNegativeID(t *testing.T) {
	lsh := newLSHCandidateIndex(32, 4, 10)
	sig := corelsh.NewMinHasher(128).ComputeSignature([]string{"feature"})
	if err := lsh.AddFragment(-1, sig); err == nil {
		t.Fatal("expected negative fragment ID to be rejected")
	}
}
