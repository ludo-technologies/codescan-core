# jscan version

Prints version information.

```bash
jscan version
```

```console
$ jscan version
jscan version 0.4.1
```

## Flags

| Flag | Short | Default | Description |
| --- | --- | --- | --- |
| `--verbose` | `-v` | `false` | Also print the commit, build date, and builder |

```console
$ jscan version --verbose
0.4.1 (commit: unknown, built: unknown, by: source)
```

The release pipeline injects only the version number through linker flags, so the other three fields keep their placeholder values even on an official release build. Treat `commit`, `built`, and `by` as unpopulated rather than as a sign that something went wrong with your install.

A binary you compiled yourself reports `dev` as its version, for the same reason. That is the expected output of `go build` without the release flags.

## The root flag

`jscan --version` prints the same short string as `jscan version` and exists because Cobra provides it automatically.

```console
$ jscan --version
jscan version 0.4.1
```
