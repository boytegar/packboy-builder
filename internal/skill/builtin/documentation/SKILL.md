---
name: documentation
description: Writing clear, maintainable documentation for Go code: package docs, function docs, README, and API reference. Use when writing or improving code documentation.
origin: builtin
---

# Documentation

## When to use
- Writing package-level documentation
- Documenting exported functions, types, and methods
- Writing README files
- Generating API reference from code

## Go documentation conventions
- Comments directly preceding declarations become documentation
- Start with the name being documented: `// Package foo does X.`
- Use complete sentences
- No markdown in godoc comments (only basic formatting)
- Examples in `_test.go` files appear in godoc

## Package documentation
```go
// Package foo provides bar processing utilities.
//
// Usage:
//
//	result := foo.Process(input)
//
// For advanced use cases, see [AdvancedProcessor].
package foo
```

## Function documentation
```go
// Process transforms the input data and returns the result.
// It returns an error if the input is empty or malformed.
func Process(input []byte) ([]byte, error)
```

## Type documentation
```go
// Config holds the configuration for the processor.
// Zero-value Config is not valid; use NewConfig to create one.
type Config struct {
    // Workers is the number of parallel workers (default: 4).
    Workers int
    // Timeout is the maximum time for a single operation.
    Timeout time.Duration
}
```

## Example tests
```go
func ExampleProcess() {
    result, _ := Process([]byte("hello"))
    fmt.Println(string(result))
    // Output: HELLO
}
```

## README structure
1. **Title + one-line description**
2. **Installation** — how to install
3. **Quick start** — minimal working example
4. **Usage** — detailed examples
5. **Configuration** — env vars, config files
6. **Contributing** — how to contribute
7. **License** — SPDX identifier

## Best practices
- **Document why, not what**: the code shows what; comments should explain why
- **Keep comments near the code**: avoid separate docs that drift
- **Update docs when you change code**: stale docs are worse than no docs
- **Use links**: `[pkg.Func]` in godoc creates cross-references
- **Document edge cases**: nil inputs, empty collections, error conditions
- **Provide examples**: a working example beats a paragraph of explanation

## Anti-patterns
- `// Process the data` (says what, not why)
- `// TODO: fix this` without context
- Large comment blocks that restate the function signature
- Documentation in separate files that drift from code
- Over-documenting trivial code (don't comment `i++`)
