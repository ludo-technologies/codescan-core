# Installation

jscan is distributed as a single self-contained binary. There is no runtime dependency to install alongside it, and it does not need your project's `node_modules` to be present.

## Run without installing

The fastest way to try jscan is to let npm fetch it for one run:

```bash
npx jscan analyze src/
```

This downloads the binary for your platform into the npm cache and runs it. Nothing is added to your project.

## Install with npm

Install jscan globally when you expect to run it regularly:

```bash
npm install -g jscan
```

Install it as a development dependency when you want every contributor and every continuous integration run to use the same version:

```bash
npm install --save-dev jscan
```

The npm package is a small launcher script. It selects the correct platform package from `optionalDependencies` and runs the binary inside it, so the download only ever includes the build for the machine doing the install.

### Supported platforms

| Operating system | Architecture | Platform package |
| --- | --- | --- |
| macOS | arm64 (Apple silicon) | `jscan-darwin-arm64` |
| Linux | x64 | `jscan-linux-x64` |
| Linux | arm64 | `jscan-linux-arm64` |
| Windows | x64 | `jscan-windows-x64` |

There is no prebuilt binary for Intel macOS or for 32-bit systems. On those machines the launcher exits with an error that names your platform, and you should build from source instead.

## Install with Go

If you already have a Go toolchain, you can install jscan directly:

```bash
go install github.com/ludo-technologies/polyscan/jscan/cmd/jscan@latest
```

This places the `jscan` binary in your `GOBIN` directory, which defaults to `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

## Build from source

Building from source requires Go 1.24.6 or later and a working C compiler. The C compiler is necessary because jscan parses with tree-sitter, which is a C library reached through cgo.

```bash
git clone https://github.com/ludo-technologies/polyscan.git
cd polyscan/jscan
go build -o jscan ./cmd/jscan
```

!!! note "Cross-compilation does not work"

    Because tree-sitter needs cgo, you cannot build a Linux binary on macOS by setting `GOOS`. Each platform binary has to be compiled on that platform. This is why the release pipeline uses a separate runner for every target.

## Verify the installation

```bash
jscan version
```

The command prints the release version:

```console
$ jscan version
jscan version 0.4.1
```

A binary you build yourself reports `dev`, because the version string is injected during the release build through linker flags.

Adding `--verbose` prints the same version together with three build metadata fields:

```console
$ jscan version --verbose
0.4.1 (commit: unknown, built: unknown, by: source)
```

The release pipeline currently injects only the version number, so the commit, build date, and builder fields keep their placeholder values on official builds as well.

## Next step

Continue to the [quick start](quick-start.md) to run your first analysis and read the report.
