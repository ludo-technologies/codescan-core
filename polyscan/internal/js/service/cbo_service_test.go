package service

import (
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func couplingClass(name, filePath string, coupling int) domain.ClassCoupling {
	return domain.ClassCoupling{
		Name:     name,
		FilePath: filePath,
		Metrics:  domain.CBOMetrics{CouplingCount: coupling},
	}
}

func reversed(classes []domain.ClassCoupling) []domain.ClassCoupling {
	out := make([]domain.ClassCoupling, len(classes))
	for i, c := range classes {
		out[len(classes)-1-i] = c
	}
	return out
}

func classKeys(classes []domain.ClassCoupling) []string {
	keys := make([]string, len(classes))
	for i, c := range classes {
		keys[i] = c.FilePath + ":" + c.Name
	}
	return keys
}

func assertSameOrder(t *testing.T, want, got []domain.ClassCoupling) {
	t.Helper()
	wantKeys, gotKeys := classKeys(want), classKeys(got)
	if len(wantKeys) != len(gotKeys) {
		t.Fatalf("length mismatch: want %d, got %d", len(wantKeys), len(gotKeys))
	}
	for i := range wantKeys {
		if wantKeys[i] != gotKeys[i] {
			t.Fatalf("order differs at %d: want %v, got %v", i, wantKeys, gotKeys)
		}
	}
}

// Classes that tie on the primary sort key must come out in the same order
// regardless of input order, which varies under concurrent analysis.
func TestCBOService_sortClasses_TieBreakIsInputOrderIndependent(t *testing.T) {
	service := NewCBOServiceWithDefaults()

	tied := []domain.ClassCoupling{
		couplingClass("Gamma", "src/c.js", 7),
		couplingClass("Alpha", "src/a.js", 7),
		couplingClass("Beta", "src/b.js", 7),
		couplingClass("Delta", "src/d.js", 9),
	}

	for _, sortBy := range []domain.SortCriteria{domain.SortByCoupling, domain.SortByName, domain.SortByRisk} {
		forward := service.sortClasses(tied, sortBy)
		backward := service.sortClasses(reversed(tied), sortBy)
		assertSameOrder(t, forward, backward)
	}
}

func TestCBOService_sortClasses_TiesOrderedBySourceLocation(t *testing.T) {
	service := NewCBOServiceWithDefaults()

	sorted := service.sortClasses([]domain.ClassCoupling{
		couplingClass("Gamma", "src/c.js", 7),
		couplingClass("Alpha", "src/a.js", 7),
		couplingClass("Beta", "src/b.js", 7),
	}, domain.SortByCoupling)

	assertSameOrder(t, []domain.ClassCoupling{
		couplingClass("Alpha", "src/a.js", 7),
		couplingClass("Beta", "src/b.js", 7),
		couplingClass("Gamma", "src/c.js", 7),
	}, sorted)
}

// A tie at the top-10 cutoff must not let ranking membership depend on input
// order: previously, whichever tied class the concurrent schedule happened to
// place earlier won the last slot.
func TestCBOService_generateSummary_MostCoupledClassesStableAtTiedCutoff(t *testing.T) {
	service := NewCBOServiceWithDefaults()

	classes := make([]domain.ClassCoupling, 0, mostCoupledClassesLimit+2)
	for i := 0; i < mostCoupledClassesLimit-1; i++ {
		classes = append(classes, couplingClass("Popular", "src/popular.js", 100+i))
	}
	// Three classes tie below the leaders; only one fits the last slot.
	classes = append(classes,
		couplingClass("TieC", "src/tie_c.js", 13),
		couplingClass("TieA", "src/tie_a.js", 13),
		couplingClass("TieB", "src/tie_b.js", 13),
	)

	forward := service.generateSummary(classes, 1, domain.CBORequest{})
	backward := service.generateSummary(reversed(classes), 1, domain.CBORequest{})

	if len(forward.MostCoupledClasses) != mostCoupledClassesLimit {
		t.Fatalf("expected %d most coupled classes, got %d", mostCoupledClassesLimit, len(forward.MostCoupledClasses))
	}
	assertSameOrder(t, forward.MostCoupledClasses, backward.MostCoupledClasses)

	last := forward.MostCoupledClasses[mostCoupledClassesLimit-1]
	if last.Name != "TieA" {
		t.Errorf("last slot should go to the tied class that precedes by source location, got %s", last.Name)
	}
}
