package analyzer

import (
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// Small function for benchmarking
var smallCode = `
function simple(x) {
    if (x > 0) {
        return x * 2;
    }
    return x;
}
`

// Medium-sized function for benchmarking
var mediumCode = `
function process(data) {
    let result = 0;
    for (let i = 0; i < data.length; i++) {
        if (data[i] > 0) {
            result += data[i];
        } else if (data[i] < -10) {
            result -= data[i];
        } else {
            continue;
        }

        if (result > 1000) {
            break;
        }
    }

    switch (result) {
        case 0:
            return "zero";
        case 1:
            return "one";
        default:
            return "other";
    }
}
`

// Large function for benchmarking
var largeCode = `
function complexFunction(input, options) {
    let result = [];
    let state = "initial";

    for (let i = 0; i < input.length; i++) {
        const item = input[i];

        if (options.filter && !options.filter(item)) {
            continue;
        }

        try {
            switch (state) {
                case "initial":
                    if (item.type === "start") {
                        state = "processing";
                    } else if (item.type === "skip") {
                        continue;
                    } else {
                        throw new Error("Invalid initial state");
                    }
                    break;

                case "processing":
                    if (item.value > 0) {
                        for (let j = 0; j < item.value; j++) {
                            if (j % 2 === 0) {
                                result.push({ type: "even", value: j });
                            } else {
                                result.push({ type: "odd", value: j });
                            }
                        }
                    } else if (item.value === 0) {
                        state = "waiting";
                    } else {
                        state = "error";
                        break;
                    }
                    break;

                case "waiting":
                    if (item.signal) {
                        state = "processing";
                    }
                    break;

                case "error":
                    if (options.recover) {
                        state = "initial";
                    } else {
                        return { error: true, partial: result };
                    }
                    break;
            }
        } catch (e) {
            if (options.throwOnError) {
                throw e;
            }
            result.push({ error: e.message });
        }
    }

    return { success: true, data: result };
}
`

func parseBenchmarkCode(t testing.TB, code string) *parser.Node {
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.Parse([]byte(code))
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	return ast
}

func findBenchmarkFunction(ast *parser.Node) *parser.Node {
	var result *parser.Node
	ast.Walk(func(n *parser.Node) bool {
		if n.Type == parser.NodeFunction || n.Type == parser.NodeArrowFunction {
			result = n
			return false
		}
		return true
	})
	return result
}

// BenchmarkCFGBuilder benchmarks CFG building for different code sizes
func BenchmarkCFGBuilder_Small(b *testing.B) {
	ast := parseBenchmarkCode(b, smallCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = builder.Build(funcNode)
	}
}

func BenchmarkCFGBuilder_Medium(b *testing.B) {
	ast := parseBenchmarkCode(b, mediumCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = builder.Build(funcNode)
	}
}

func BenchmarkCFGBuilder_Large(b *testing.B) {
	ast := parseBenchmarkCode(b, largeCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = builder.Build(funcNode)
	}
}

// BenchmarkComplexityCalculation benchmarks complexity analysis
func BenchmarkComplexityCalculation_Small(b *testing.B) {
	ast := parseBenchmarkCode(b, smallCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateComplexity(cfg)
	}
}

func BenchmarkComplexityCalculation_Medium(b *testing.B) {
	ast := parseBenchmarkCode(b, mediumCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateComplexity(cfg)
	}
}

func BenchmarkComplexityCalculation_Large(b *testing.B) {
	ast := parseBenchmarkCode(b, largeCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateComplexity(cfg)
	}
}

func BenchmarkComplexityCalculationWithConfig(b *testing.B) {
	ast := parseBenchmarkCode(b, mediumCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	complexityConfig := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateComplexityWithConfig(cfg, complexityConfig)
	}
}

// BenchmarkReachabilityAnalysis benchmarks reachability analysis
func BenchmarkReachabilityAnalysis_Small(b *testing.B) {
	ast := parseBenchmarkCode(b, smallCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)
	analyzer := NewReachabilityAnalyzer(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.AnalyzeReachability()
	}
}

func BenchmarkReachabilityAnalysis_Large(b *testing.B) {
	ast := parseBenchmarkCode(b, largeCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)
	analyzer := NewReachabilityAnalyzer(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.AnalyzeReachability()
	}
}

// BenchmarkDeadCodeDetection benchmarks dead code detection
func BenchmarkDeadCodeDetection_Small(b *testing.B) {
	ast := parseBenchmarkCode(b, smallCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector := NewDeadCodeDetector(cfg)
		_ = detector.Detect()
	}
}

func BenchmarkDeadCodeDetection_Large(b *testing.B) {
	ast := parseBenchmarkCode(b, largeCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector := NewDeadCodeDetector(cfg)
		_ = detector.Detect()
	}
}

// BenchmarkCFGTraversal benchmarks CFG traversal
func BenchmarkCFGTraversal_Walk(b *testing.B) {
	ast := parseBenchmarkCode(b, largeCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	visitor := &noOpVisitor{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Walk(visitor)
	}
}

func BenchmarkCFGTraversal_BreadthFirst(b *testing.B) {
	ast := parseBenchmarkCode(b, largeCode)
	funcNode := findBenchmarkFunction(ast)
	if funcNode == nil {
		b.Fatal("Function not found")
	}

	builder := NewCFGBuilder()
	cfg, _ := builder.Build(funcNode)

	visitor := &noOpVisitor{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.BreadthFirstWalk(visitor)
	}
}

type noOpVisitor struct{}

func (v *noOpVisitor) VisitBlock(block *BasicBlock) bool { return true }
func (v *noOpVisitor) VisitEdge(edge *Edge) bool         { return true }
