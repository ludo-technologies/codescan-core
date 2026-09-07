package analysis

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ludo-technologies/polyscan/core/cbo"
	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	"github.com/ludo-technologies/polyscan/polyscan/internal/godeps"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang/golang"
)

// CoupledClass is the coupling result for one type.
type CoupledClass struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Language string `json:"language"`
	// StartLine and EndLine span the type's declaration.
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
	// CBO is the number of distinct types of the analyzed tree that the
	// type's declaration and methods refer to. Types outside the tree, such
	// as the standard library's, do not count.
	CBO              int      `json:"cbo"`
	DependentClasses []string `json:"dependent_classes"`
	// Inheritance counts the references that embed or implement another
	// type; TypeHint every other reference.
	Inheritance int              `json:"inheritance"`
	TypeHint    int              `json:"type_hint"`
	IsAbstract  bool             `json:"is_abstract"`
	RiskLevel   domain.RiskLevel `json:"risk_level"`
}

// Coupling is the CBO analysis of a set of paths.
type Coupling struct {
	// Classes lists every type coupled to at least one other, sorted by
	// descending CBO, then by location.
	Classes       []CoupledClass `json:"classes"`
	FilesAnalyzed int            `json:"files_analyzed"`
	Warnings      []string       `json:"warnings,omitempty"`
}

// couplingFile is what the coupling analysis keeps of one file.
type couplingFile struct {
	language *engine.Language
	display  string
	// location identifies the unit a type belongs to: the directory when
	// the language spreads a type over one, otherwise the file.
	location string
	// importPath, pkg and imports are set for Go files inside a module.
	importPath string
	pkg        string
	imports    []godeps.Import
	types      []engine.Type
}

// coupledType accumulates one type of the tree before it is measured.
type coupledType struct {
	key      string
	name     string
	language *engine.Language
	file     string
	start    int
	end      int
	declared bool
	abstract bool
	refs     []siteReference
}

// siteReference is a reference together with the file that makes it, which
// decides how the name resolves.
type siteReference struct {
	file *couplingFile
	ref  engine.Reference
}

// couplingBuilder collects the types of every file and resolves their
// references once the whole tree is known.
type couplingBuilder struct {
	files    []*couplingFile
	resolver *godeps.Resolver
	noModule map[string]bool
	warnings []string
}

func newCouplingBuilder() *couplingBuilder {
	return &couplingBuilder{resolver: godeps.NewResolver(), noModule: map[string]bool{}}
}

// add records the types of one file. A Go file also contributes its package
// clause and imports, through which qualified references resolve; a Go file
// with no go.mod above it keeps its unqualified references only, with one
// warning per directory.
func (b *couplingBuilder) add(language *engine.Language, display, path string, content []byte, result *engine.Result) error {
	file := &couplingFile{language: language, display: display, location: display}
	if language.TypeSpansDirectory {
		file.location = filepath.Dir(display)
	}
	for _, t := range result.Types {
		if !t.IsTest {
			file.types = append(file.types, t)
		}
	}
	if language == golang.Language {
		info, err := godeps.ParseFile(content)
		if err != nil {
			b.warnings = append(b.warnings, fmt.Sprintf("%s: %v; its types are absent from the coupling analysis", display, err))
			return nil
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(absolute)
		importPath, noModule, err := b.resolver.ImportPath(dir)
		if err != nil {
			return err
		}
		if noModule && !b.noModule[dir] {
			b.noModule[dir] = true
			b.warnings = append(b.warnings, fmt.Sprintf("%s: no go.mod above it; references to types of other packages are not resolved", file.location))
		}
		file.importPath, file.pkg, file.imports = importPath, info.Package, info.Imports
	}
	b.files = append(b.files, file)
	return nil
}

// build resolves every reference and measures the types. A reference counts
// when it names a type the tree declares: for Go an unqualified name is a
// type of the same package and a qualified one is looked up through the
// file's imports; for Rust a bare name is a type of the same file, or else
// of any file. A type whose methods a file holds without declaring it, as a
// Go type is across its package and a Rust type across impl blocks, joins
// its declaration; a Rust impl for a type no file declares is left out.
func (b *couplingBuilder) build() *Coupling {
	types := map[string]*coupledType{}
	var order []string
	typeOf := func(file *couplingFile, name string) *coupledType {
		key := file.language.Name + "\x00" + file.location + "\x00" + name
		t, ok := types[key]
		if !ok {
			t = &coupledType{key: key, name: name, language: file.language}
			types[key] = t
			order = append(order, key)
		}
		return t
	}
	// byImportPath maps a Go package's import path and type name to the
	// declared type; byBare maps a language and bare type name to the
	// declared types of the whole tree, byFileBare to those of one file.
	byImportPath := map[string]*coupledType{}
	packageNames := map[string]string{}
	byBare := map[string][]*coupledType{}
	byFileBare := map[*couplingFile]map[string]*coupledType{}
	bareKey := func(language *engine.Language, name string) string {
		return language.Name + "\x00" + bareName(language, name)
	}

	for _, file := range b.files {
		if file.importPath != "" {
			packageNames[file.importPath] = file.pkg
		}
		byFileBare[file] = map[string]*coupledType{}
		for _, et := range file.types {
			t := typeOf(file, et.Name)
			if et.Declared && !t.declared {
				t.declared = true
				t.file, t.start, t.end = file.display, et.StartLine, et.EndLine
				t.abstract = et.Abstract
				if file.importPath != "" {
					byImportPath[file.importPath+"."+et.Name] = t
				}
				bare := bareKey(file.language, et.Name)
				byBare[bare] = append(byBare[bare], t)
				byFileBare[file][bareName(file.language, et.Name)] = t
			}
			for _, ref := range et.References {
				t.refs = append(t.refs, siteReference{file: file, ref: ref})
			}
		}
	}

	// A Rust impl block or a method group in a file that does not declare
	// its type belongs to the declaration elsewhere when there is exactly
	// one.
	for _, key := range order {
		t := types[key]
		if t.declared || t.language.TypeSpansDirectory {
			continue
		}
		if candidates := byBare[bareKey(t.language, t.name)]; len(candidates) == 1 {
			candidates[0].refs = append(candidates[0].refs, t.refs...)
		}
	}

	resolve := func(site siteReference) (*coupledType, string) {
		file, ref := site.file, site.ref
		switch {
		case ref.Package != "":
			target, ok := b.resolveQualified(file, ref, byImportPath, packageNames)
			if !ok {
				return nil, ""
			}
			return target, ref.Package + "." + ref.Name
		case file.language.TypeSpansDirectory:
			target, ok := types[file.language.Name+"\x00"+file.location+"\x00"+ref.Name]
			if !ok || !target.declared {
				return nil, ""
			}
			return target, ref.Name
		default:
			if target, ok := byFileBare[file][ref.Name]; ok {
				return target, ref.Name
			}
			if candidates := byBare[bareKey(file.language, ref.Name)]; len(candidates) > 0 {
				return candidates[0], ref.Name
			}
			return nil, ""
		}
	}

	// ComputeCBO returns one result per class, in order.
	var classes []*cbo.ClassInfo
	var owners []*coupledType
	for _, key := range order {
		t := types[key]
		if !t.declared {
			continue
		}
		class := &cbo.ClassInfo{
			Name:       t.name,
			FilePath:   t.file,
			StartLine:  t.start,
			EndLine:    t.end,
			IsAbstract: t.abstract,
		}
		// The breakdown counts dependencies, so a type referred to twice
		// in the same way is one dependency.
		seen := map[cbo.ClassDependency]bool{}
		for _, site := range t.refs {
			target, display := resolve(site)
			if target == nil || target == t {
				continue
			}
			dep := cbo.ClassDependency{ClassName: display, Kind: cbo.DepTypeHint}
			if site.ref.Embedded {
				dep.Kind = cbo.DepInheritance
			}
			if !seen[dep] {
				seen[dep] = true
				class.Dependencies = append(class.Dependencies, dep)
			}
		}
		classes = append(classes, class)
		owners = append(owners, t)
	}

	coupling := &Coupling{Classes: []CoupledClass{}, FilesAnalyzed: len(b.files), Warnings: b.warnings}
	for i, result := range cbo.ComputeCBO(classes, cbo.DefaultConfig()) {
		if result.CouplingCount == 0 {
			continue
		}
		coupling.Classes = append(coupling.Classes, CoupledClass{
			Name:             result.ClassName,
			FilePath:         result.FilePath,
			Language:         owners[i].language.Name,
			StartLine:        result.StartLine,
			EndLine:          result.EndLine,
			CBO:              result.CouplingCount,
			DependentClasses: result.DependentClasses,
			Inheritance:      result.DependencyBreakdown[cbo.DepInheritance],
			TypeHint:         result.DependencyBreakdown[cbo.DepTypeHint],
			IsAbstract:       result.IsAbstract,
			RiskLevel:        result.RiskLevel,
		})
	}
	sort.SliceStable(coupling.Classes, func(i, j int) bool {
		a, b := coupling.Classes[i], coupling.Classes[j]
		if a.CBO != b.CBO {
			return a.CBO > b.CBO
		}
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		return a.StartLine < b.StartLine
	})
	return coupling
}

// resolveQualified finds the type a Go reference pkg.T names: the file's
// import whose name is pkg, explicit or the package's own, must lead to a
// package of the tree that declares T.
func (b *couplingBuilder) resolveQualified(file *couplingFile, ref engine.Reference, byImportPath map[string]*coupledType, packageNames map[string]string) (*coupledType, bool) {
	for _, imported := range file.imports {
		name := imported.Name
		switch name {
		case "_", ".":
			continue
		case "":
			name = packageNames[imported.Path]
		}
		if name != ref.Package {
			continue
		}
		target, ok := byImportPath[imported.Path+"."+ref.Name]
		return target, ok
	}
	return nil, false
}

// bareName strips the scope prefix from a type name.
func bareName(language *engine.Language, name string) string {
	if i := strings.LastIndex(name, language.Separator()); i >= 0 {
		return name[i+len(language.Separator()):]
	}
	return name
}
