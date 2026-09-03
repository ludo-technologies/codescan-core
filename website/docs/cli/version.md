# polyscan version

Prints version information.

```bash
polyscan version
```

```console
$ polyscan version
polyscan version 0.1.0
```

## Flags

| Flag | Short | Default | Description |
| --- | --- | --- | --- |
| `--verbose` | `-v` | `false` | Also print the commit, build date, and builder |

```console
$ polyscan version --verbose
0.1.0 (commit: 7b3d1e2, built: 2026-08-14T07:40:09Z, by: release)
```

Official release builds inject the version tag, commit hash, build timestamp in UTC, and `by: release`.

A binary you compiled yourself using bare `go build` reports placeholder values (`dev`, commit `unknown`, built `unknown`, by `source`). Building via `make` sets `by: make` and local git metadata.

## The root flag

`polyscan --version` prints the same short string as `polyscan version` and exists because Cobra provides it automatically.

```console
$ polyscan --version
polyscan version 0.1.0
```
