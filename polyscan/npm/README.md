# polyscan

Multi-language code quality analyzer. It measures cyclomatic complexity and detects code clones for Go and Rust.

## Installation

```bash
npm install -g polyscan
```

Or use with npx:

```bash
npx polyscan analyze .
```

## Usage

```bash
# HTML report, opened in the browser
polyscan analyze .

# JSON report
polyscan analyze --format json src/
```

## Documentation

For full documentation, visit [GitHub](https://github.com/ludo-technologies/polyscan/tree/main/polyscan).
