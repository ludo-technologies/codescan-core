// Package app holds reusable use cases that sit between the CLI and the
// service layer.
//
// The use cases here bundle the steps a command would otherwise repeat:
// collecting files, building a domain request, invoking a service, and
// formatting the result. AnalyzeUseCase covers the full pipeline, while
// ComplexityUseCase and DeadCodeUseCase cover a single analysis each.
//
// Not every entry point routes through this package. js.Run calls services
// directly when doing so makes the concurrency clearer, which is why polyscan
// analyze runs the five analyses itself rather than through AnalyzeUseCase.
//
// FileHelper is the piece every entry point does share. Its CollectJSFiles walks
// the given paths and returns the JavaScript and TypeScript files to analyze.
// Two behaviors of that walk are worth knowing, because both silently reduce
// the input:
//
//   - A .gitignore at the root of the walked directory is honored. Only that
//     one file is read, not nested ones and not the repository root's when a
//     subdirectory was passed.
//   - Exclude patterns are matched on whole path segments, ignoring case and
//     relative to the analysis root, so "dist" skips dist/bundle.js and leaves
//     src/utils/distance.ts alone.
//   - Include patterns, when a caller supplies any, keep only the files that
//     match one, under the same matching rules. They narrow the walk; they
//     cannot widen it, because the analyzed extensions are fixed in isJSFile.
//
// Both pattern lists apply to directory walks. A file named directly on the
// command line skips include filtering, since dropping a file the user asked
// for by name would be the wrong answer, and is matched against exclude
// patterns on its own name alone.
package app
