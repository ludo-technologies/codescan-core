package godeps

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

// pkg accumulates the files of one package directory.
type pkg struct {
	importPath string
	dir        string
	name       string
	// imports counts, per imported path, the files that import it.
	imports            map[string]int
	exportedTypes      int
	exportedInterfaces int
}

// Build builds the package graph of files. Every file must be a Go source and
// is placed in the package of its directory; files in a directory the go tool
// ignores, such as vendor and testdata, are ignored too. A file whose
// directory lies in no module, or that cannot be read or parsed, is left out
// and reported in the warnings. display shortens a directory for the report.
//
// Edges join packages of the tree only: an import of the standard library, of
// another module or of a vendored copy names no node and is dropped. Each
// pair of packages is joined by one edge whose weight is the number of files
// that import the target.
func Build(files []string, display func(string) string) (*domain.DependencyGraph, []string) {
	var warnings []string
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}

	finder := newModuleFinder()
	packages := map[string]*pkg{}
	noModule := map[string]bool{}
	sorted := append([]string{}, files...)
	sort.Strings(sorted)
	for _, file := range sorted {
		absolute, err := filepath.Abs(file)
		if err != nil {
			warn("%s: %v", display(file), err)
			continue
		}
		dir := filepath.Dir(absolute)
		m, err := finder.find(dir)
		if err != nil {
			warn("%s: %v", display(file), err)
			continue
		}
		if m == nil {
			if !noModule[dir] {
				noModule[dir] = true
				warn("%s: no go.mod above it; its packages are absent from the dependency graph", display(dir))
			}
			continue
		}
		importPath, ignored, err := m.importPath(dir)
		if err != nil {
			warn("%s: %v", display(file), err)
			continue
		}
		if ignored {
			continue
		}
		content, err := os.ReadFile(absolute)
		if err != nil {
			warn("%s: %v", display(file), err)
			continue
		}
		info, err := parseFile(content)
		if err != nil {
			warn("%s: %v; excluded from the dependency graph", display(file), err)
			continue
		}
		p, ok := packages[importPath]
		if !ok {
			p = &pkg{importPath: importPath, dir: display(dir), name: info.Package, imports: map[string]int{}}
			packages[importPath] = p
		} else if p.name != info.Package {
			warn("%s: package %s differs from package %s in the same directory; reported as %s", display(file), info.Package, p.name, p.name)
		}
		seen := map[string]bool{}
		for _, imported := range info.Imports {
			if !seen[imported] {
				seen[imported] = true
				p.imports[imported]++
			}
		}
		p.exportedTypes += info.ExportedTypes
		p.exportedInterfaces += info.ExportedInterfaces
	}

	graph := domain.NewDependencyGraph()
	for _, p := range packages {
		graph.AddNode(&domain.ModuleNode{
			ID:           p.importPath,
			Name:         p.name,
			FilePath:     p.dir,
			ModuleType:   domain.ModuleTypePackage,
			Abstractness: abstractness(p),
		})
	}
	for _, from := range graph.NodeIDs() {
		p := packages[from]
		targets := make([]string, 0, len(p.imports))
		for to := range p.imports {
			if graph.HasNode(to) && to != from {
				targets = append(targets, to)
			}
		}
		sort.Strings(targets)
		for _, to := range targets {
			graph.AddEdge(&domain.DependencyEdge{
				From:     from,
				To:       to,
				EdgeType: domain.EdgeTypeImport,
				Weight:   p.imports[to],
			})
		}
	}
	graph.UpdateNodeFlags()
	return graph, warnings
}

// abstractness is Martin's A for a Go package: the share of its exported type
// declarations that are interfaces. A package exporting no type is concrete.
func abstractness(p *pkg) float64 {
	if p.exportedTypes == 0 {
		return 0
	}
	return float64(p.exportedInterfaces) / float64(p.exportedTypes)
}
