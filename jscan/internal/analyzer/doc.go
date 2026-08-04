// Package analyzer holds jscan's analysis engines. Everything that inspects
// parsed JavaScript or TypeScript and produces a finding lives here.
//
// The language-neutral algorithms are imported from polyscan/core and shared
// with pyscn. What stays in this package is the JavaScript and TypeScript
// knowledge those algorithms need: how the language's statements affect control
// flow, how its modules resolve, and what its syntax costs in a tree
// comparison.
//
// # Control flow
//
// CFGBuilder turns a parsed function into a core/cfg.CFG. The shared package
// then computes reachability, McCabe complexity, and dead code over that graph,
// while javaScriptCFGClassifier supplies the language-specific answers about
// which statements terminate a block and which are no-ops.
//
// One consequence of the current builder is worth knowing: a switch statement
// produces a single branch regardless of how many cases it has, so a four-case
// switch scores 2 where an equivalent chain of four if statements scores 5.
//
// # Dead code
//
// Two independent passes feed the dead code results. dead_code.go and
// reachability.go find statements unreachable within one function, all of which
// are reported as critical. unused_code.go compares imports against exports
// across every file in the run to find unused imports, unused exports, and
// orphan files. That second pass can only see the files it was given, which is
// why analyzing a subdirectory reports its exports as unused.
//
// unused_code.go also carries the framework exemptions. Next.js App Router
// convention files keep their default export and the framework's reserved
// export names without being reported.
//
// # Clone detection
//
// clone_detector.go finds duplicate code in two stages. MinHash fingerprints
// and locality-sensitive hashing from core/lsh narrow the candidate pairs,
// after which APTED tree edit distance from core/apted compares the survivors.
// The kernels are shared; apted_tree.go contributes the parser-to-tree
// converter, apted_cost.go the JavaScript cost model, and javascript_comments.go
// the comment stripper that core/clone injects.
//
// # Modules and coupling
//
// module_analyzer.go resolves ECMAScript and CommonJS imports and exports, and
// dependency_graph.go assembles them into the module graph. Note that
// TypeScript path aliases such as "@/lib/util" are not resolved: each becomes a
// module node of its own rather than pointing at the file it refers to, which
// inflates module counts and hides cycles that pass through an alias.
//
// cbo.go measures coupling. Despite the name it produces one entry per file
// rather than per class, named after the module.
// circular_detector.go enriches core/graph's Tarjan results, excluding dynamic
// imports because those do not create a load-time cycle, and
// coupling_metrics.go layers the Martin metrics on top.
package analyzer
