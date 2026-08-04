// Package app holds reusable use cases that sit between the CLI and the
// service layer.
//
// The use cases here bundle the steps a command would otherwise repeat:
// collecting files, building a domain request, invoking a service, and
// formatting the result. AnalyzeUseCase covers the full pipeline, while
// ComplexityUseCase and DeadCodeUseCase cover a single analysis each.
//
// Not every command routes through this package. Handlers in cmd/jscan call
// services directly when doing so makes the concurrency clearer, which is why
// the analyze command runs its five analyses itself rather than through
// AnalyzeUseCase.
//
// FileHelper is the piece every command does share. Its CollectJSFiles walks
// the given paths and returns the JavaScript and TypeScript files to analyze.
// Two behaviors of that walk are worth knowing, because both silently reduce
// the input:
//
//   - A .gitignore at the root of the walked directory is honored. Only that
//     one file is read, not nested ones and not the repository root's when a
//     subdirectory was passed.
//   - Exclude patterns are applied to directories by exact name or glob, but to
//     files by glob on the base name OR by plain substring match against the
//     whole path. The substring rule is broad: the default pattern "out"
//     removes src/routes/api.ts and src/layout/Header.tsx, and "dist" removes
//     src/utils/distance.ts.
//
// The include patterns parameter is accepted for interface compatibility and
// ignored. The set of analyzed extensions is fixed in isJSFile.
package app
