---
name: testing-strategy
description: Comprehensive testing methodology for Go: unit tests, integration tests, table-driven tests, benchmarks, fuzz testing, and test coverage. Use when writing tests, planning test strategy, or improving test quality.
origin: builtin
---

# Testing Strategy

## Purpose
Design and implement a testing strategy that catches bugs early, documents behavior, and gives confidence to refactor.

## When to use
- Writing new tests for Go code
- Planning test coverage strategy
- Improving existing test quality
- Setting up CI test pipelines

## Test pyramid
1. **Unit tests** — fast, isolated, test individual functions/types. Most tests go here.
2. **Integration tests** — slower, test multiple components together. Fewer than unit tests.
3. **End-to-end tests** — slowest, test the full system from the outside. Very few.

## Go testing patterns

### Table-driven tests
```go
func TestCalculate(t *testing.T) {
    tests := []struct {
        name     string
        input    int
        expected int
    }{
        {"zero", 0, 0},
        {"positive", 5, 25},
        {"negative", -3, 9},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Calculate(tt.input)
            if got != tt.expected {
                t.Errorf("Calculate(%d) = %d, want %d", tt.input, got, tt.expected)
            }
        })
    }
}
```

### Subtests with t.Run
- Use subtests for logical grouping
- Run individual subtests with `go test -run TestName/subtest`

### Test helpers
- `t.Helper()` marks helper functions — error reporting points to the caller
- `t.Cleanup()` for deferred cleanup (replaces defer in subtests)

### Benchmarks
- Use `BenchmarkXxx(b *testing.B)` functions
- Always use `b.ResetTimer()` after setup
- Use `b.ReportAllocs()` to track allocations
- Run with `go test -bench=. -benchmem`

### Fuzz testing
- Use Go 1.18+ native fuzzing: `func FuzzXxx(f *testing.F)`
- Seed with known edge cases
- Run with `go test -fuzz=.`
- Fuzz targets: parsers, decoders, input processors

## What to test
- **Happy path**: normal inputs, expected outputs
- **Edge cases**: empty, zero, nil, max, negative, boundary values
- **Error cases**: invalid input, missing required fields, resource exhaustion
- **Concurrency**: race conditions with `-race` flag
- **Idempotency**: running twice produces the same result

## Test isolation
- Each test should be independent — no shared mutable state
- Use setup/teardown per test, not per suite
- Mock external dependencies (network, filesystem, time)
- Use `t.TempDir()` for filesystem isolation

## Mocking
- Prefer interfaces for testability — define at the consumer side
- Use simple hand-written mocks for most cases
- Use mockery or mockgen for complex interfaces
- Don't mock what you don't own — wrap external APIs in an interface

## Coverage
- Aim for high coverage on core logic, not on boilerplate
- Use `go test -cover -coverprofile=coverage.out`
- Review coverage with `go tool cover -html=coverage.out`
- Don't chase 100% — test what matters

## Anti-patterns
- Testing implementation details instead of behavior
- Brittle tests that break on unrelated changes
- Tests that don't assert anything (smoke tests)
- Tests that depend on test execution order
- Testing the mock instead of the real thing

## Test naming
- `TestFunctionName_Scenario_ExpectedResult`
- Describe what is being tested, not what the test does

## Integration tests
- Use build tags: `//go:build integration`
- Run separately: `go test -tags=integration ./...`
- Test against real databases/queues when possible
- Use testcontainers for ephemeral dependencies
