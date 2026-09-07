package clone

import (
	"math"
	"testing"

	"github.com/ludo-technologies/polyscan/core/domain"
)

func testClassifierConfig() ClassifierConfig {
	config := DefaultClassifierConfig()
	config.Type1Threshold = 0.95
	config.Type2Threshold = 0.85
	config.Type3Threshold = 0.80
	config.Type4Threshold = 0.75
	return config
}

func newTestPairClassifier() *PairClassifier {
	return NewPairClassifier(
		testClassifierConfig(),
		NewTextualSimilarityAnalyzer(nil),
		NewSyntacticSimilarityAnalyzer(),
	)
}

func TestClassifyPairType1RequiresExactTextualMatch(t *testing.T) {
	c := newTestPairClassifier()

	f1 := &CodeFragment{ID: 1, Content: "return a + b;"}
	f2 := &CodeFragment{ID: 2, Content: "return  a +  b;"} // whitespace only

	cloneType, similarity := c.ClassifyPair(f1, f2, 0.99)
	if cloneType != domain.Type1Clone {
		t.Fatalf("expected Type1, got %v", cloneType)
	}
	if similarity != 0.99 {
		t.Errorf("Type-1 similarity must be uncapped, got %f", similarity)
	}
}

func TestClassifyPairHighStructuralWithoutTextualMatchIsCapped(t *testing.T) {
	c := newTestPairClassifier()

	// Same structure, renamed identifier: structural similarity is high but
	// there is no exact textual match, so Type-1 similarity must not be
	// reported. Identical features satisfy the Type-2 syntactic gate.
	f1 := &CodeFragment{ID: 1, Content: "return alpha;", Features: []string{"f1", "f2"}}
	f2 := &CodeFragment{ID: 2, Content: "return beta;", Features: []string{"f1", "f2"}}

	cloneType, similarity := c.ClassifyPair(f1, f2, 0.99)
	if cloneType != domain.Type2Clone {
		t.Fatalf("expected Type2, got %v", cloneType)
	}
	if similarity >= c.config.Type1Threshold {
		t.Errorf("non-textual pair must report similarity below Type-1 threshold, got %f", similarity)
	}
}

func TestClassifyPairType2RequiresSyntacticGate(t *testing.T) {
	c := newTestPairClassifier()

	// High structural similarity but disjoint normalized features: the
	// syntactic gate fails and classification falls through to Type-3.
	f1 := &CodeFragment{ID: 1, Content: "if (a) { f(); }", Features: []string{"f1", "f2"}}
	f2 := &CodeFragment{ID: 2, Content: "if (b) { g(); }", Features: []string{"g1", "g2"}}

	cloneType, _ := c.ClassifyPair(f1, f2, 0.90)
	if cloneType != domain.Type3Clone {
		t.Fatalf("expected Type3 when syntactic gate fails, got %v", cloneType)
	}
}

func TestClassifyPairType2SimilarityIsMinOfGates(t *testing.T) {
	config := testClassifierConfig()
	c := NewPairClassifier(config, NewTextualSimilarityAnalyzer(nil), NewSyntacticSimilarityAnalyzer())

	// Feature overlap of 6/7 ≈ 0.857 passes the 0.85 syntactic gate and is
	// lower than the capped structural similarity, so it wins the min().
	shared := []string{"a", "b", "c", "d", "e", "f"}
	f1 := &CodeFragment{ID: 1, Content: "x", Features: append([]string{}, shared...)}
	f2 := &CodeFragment{ID: 2, Content: "y", Features: append(append([]string{}, shared...), "g")}

	cloneType, similarity := c.ClassifyPair(f1, f2, 0.99)
	if cloneType != domain.Type2Clone {
		t.Fatalf("expected Type2, got %v", cloneType)
	}
	want := 6.0 / 7.0
	if math.Abs(similarity-want) > 1e-9 {
		t.Errorf("expected min(structural, syntactic) = %f, got %f", want, similarity)
	}
}

func TestClassifyPairType3AndType4(t *testing.T) {
	c := newTestPairClassifier()

	f1 := &CodeFragment{ID: 1, Content: "a"}
	f2 := &CodeFragment{ID: 2, Content: "b"}

	if cloneType, _ := c.ClassifyPair(f1, f2, 0.82); cloneType != domain.Type3Clone {
		t.Errorf("expected Type3 at 0.82, got %v", cloneType)
	}
	if cloneType, _ := c.ClassifyPair(f1, f2, 0.76); cloneType != domain.Type4Clone {
		t.Errorf("expected Type4 at 0.76, got %v", cloneType)
	}
}

func TestClassifyPairBelowAllThresholds(t *testing.T) {
	c := newTestPairClassifier()

	f1 := &CodeFragment{ID: 1, Content: "a"}
	f2 := &CodeFragment{ID: 2, Content: "b"}

	cloneType, similarity := c.ClassifyPair(f1, f2, 0.5)
	if cloneType != 0 {
		t.Errorf("expected no clone type below all thresholds, got %v", cloneType)
	}
	if similarity != 0.5 {
		t.Errorf("similarity below Type-1 threshold must pass through unchanged, got %f", similarity)
	}
}

func TestClassifyPairDisabledTypesAreSkipped(t *testing.T) {
	config := testClassifierConfig()
	config.EnableType1 = false
	config.EnableType2 = false
	config.EnableType3 = false
	c := NewPairClassifier(config, NewTextualSimilarityAnalyzer(nil), NewSyntacticSimilarityAnalyzer())

	f1 := &CodeFragment{ID: 1, Content: "same", Features: []string{"f"}}
	f2 := &CodeFragment{ID: 2, Content: "same", Features: []string{"f"}}

	cloneType, _ := c.ClassifyPair(f1, f2, 0.99)
	if cloneType != domain.Type4Clone {
		t.Errorf("expected Type4 with types 1-3 disabled, got %v", cloneType)
	}
}

func TestCapNonTextualSimilarity(t *testing.T) {
	c := newTestPairClassifier()

	// Below the Type-1 threshold: unchanged.
	if got := c.CapNonTextualSimilarity(0.90); got != 0.90 {
		t.Errorf("below-threshold similarity must pass through, got %f", got)
	}

	// At or above the Type-1 threshold: capped just below it.
	got := c.CapNonTextualSimilarity(1.0)
	if got >= c.config.Type1Threshold {
		t.Errorf("capped similarity %f must be below Type-1 threshold %f", got, c.config.Type1Threshold)
	}
	if got < c.config.Type2Threshold {
		t.Errorf("capped similarity %f must not fall below Type-2 threshold %f", got, c.config.Type2Threshold)
	}
}

func TestPassesJaccardPreFilter(t *testing.T) {
	c := newTestPairClassifier()

	sharedFeatures := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	similar1 := &CodeFragment{ID: 1, Features: append([]string{}, sharedFeatures...)}
	similar2 := &CodeFragment{ID: 2, Features: append(append([]string{}, sharedFeatures...), "j")}
	if !c.PassesJaccardPreFilter(similar1, similar2) {
		t.Error("highly similar features must pass the pre-filter")
	}

	disjoint := &CodeFragment{ID: 3, Features: []string{"x", "y", "z", "w", "v", "u", "t", "s", "r", "q"}}
	if c.PassesJaccardPreFilter(similar1, disjoint) {
		t.Error("disjoint features must be rejected by the pre-filter")
	}

	noFeatures := &CodeFragment{ID: 4}
	if !c.PassesJaccardPreFilter(similar1, noFeatures) {
		t.Error("fragments without features must always pass")
	}

	zeroConfig := testClassifierConfig()
	zeroConfig.JaccardPreFilterThreshold = 0
	czero := NewPairClassifier(zeroConfig, nil, nil)
	if !czero.PassesJaccardPreFilter(similar1, disjoint) {
		t.Error("zero threshold must disable the pre-filter")
	}
}

func TestShouldCompareFragments(t *testing.T) {
	base := &CodeFragment{ID: 1, NodeCount: 100, LineCount: 20}

	similar := &CodeFragment{ID: 2, NodeCount: 110, LineCount: 22}
	if !ShouldCompareFragments(base, similar) {
		t.Error("similar-size fragments must be compared")
	}

	tooBig := &CodeFragment{ID: 3, NodeCount: 300, LineCount: 21}
	if ShouldCompareFragments(base, tooBig) {
		t.Error("fragments with >50% node-count difference must be skipped")
	}

	tooManyLines := &CodeFragment{ID: 4, NodeCount: 105, LineCount: 60}
	if ShouldCompareFragments(base, tooManyLines) {
		t.Error("fragments with large line-count difference must be skipped")
	}
}

func TestCalculateConfidence(t *testing.T) {
	f1 := &CodeFragment{ID: 1, NodeCount: 50, Complexity: 4}
	f2 := &CodeFragment{ID: 2, NodeCount: 50, Complexity: 4}

	confidence := CalculateConfidence(f1, f2, 0.8)
	if confidence <= 0.8 {
		t.Errorf("size and complexity bonuses should raise confidence above similarity, got %f", confidence)
	}
	if confidence > 1.0 {
		t.Errorf("confidence must be capped at 1.0, got %f", confidence)
	}

	huge1 := &CodeFragment{ID: 3, NodeCount: 10000, Complexity: 10}
	huge2 := &CodeFragment{ID: 4, NodeCount: 10000, Complexity: 10}
	if confidence := CalculateConfidence(huge1, huge2, 0.99); confidence != 1.0 {
		t.Errorf("confidence must be capped at 1.0, got %f", confidence)
	}
}

func TestLocationsOverlap(t *testing.T) {
	tests := []struct {
		name string
		a, b ItemLocation
		want bool
	}{
		{
			"different files",
			ItemLocation{FilePath: "a.js", StartLine: 1, EndLine: 10},
			ItemLocation{FilePath: "b.js", StartLine: 1, EndLine: 10},
			false,
		},
		{
			"overlapping ranges",
			ItemLocation{FilePath: "a.js", StartLine: 1, EndLine: 10},
			ItemLocation{FilePath: "a.js", StartLine: 5, EndLine: 15},
			true,
		},
		{
			"containment",
			ItemLocation{FilePath: "a.js", StartLine: 1, EndLine: 100},
			ItemLocation{FilePath: "a.js", StartLine: 5, EndLine: 15},
			true,
		},
		{
			"adjacent but disjoint",
			ItemLocation{FilePath: "a.js", StartLine: 1, EndLine: 10},
			ItemLocation{FilePath: "a.js", StartLine: 11, EndLine: 20},
			false,
		},
		{
			"shared boundary line",
			ItemLocation{FilePath: "a.js", StartLine: 1, EndLine: 10},
			ItemLocation{FilePath: "a.js", StartLine: 10, EndLine: 20},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LocationsOverlap(tt.a, tt.b); got != tt.want {
				t.Errorf("LocationsOverlap = %v, want %v", got, tt.want)
			}
			if got := LocationsOverlap(tt.b, tt.a); got != tt.want {
				t.Errorf("LocationsOverlap (reversed) = %v, want %v", got, tt.want)
			}
		})
	}
}
