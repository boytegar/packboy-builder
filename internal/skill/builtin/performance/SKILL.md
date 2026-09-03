---
name: performance
description: Performance optimization techniques for Go: profiling, benchmarking, memory allocation, goroutine pooling, and I/O optimization. Use when optimizing hot paths, reducing memory usage, or improving throughput.
origin: builtin
---

# Performance Optimization

## Purpose
Identify and fix performance bottlenecks using data-driven profiling, not guessing.

## When to use
- Application is too slow
- Memory usage is too high
- Throughput is insufficient
- User-perceived latency needs improvement

## Core principle
**Measure first, optimize second.** Never optimize based on intuition. Profile, find the bottleneck, optimize, re-measure, verify the improvement.

## Profiling tools

### CPU profiling
```bash
go test -cpuprofile=cpu.out -bench=.
go tool pprof cpu.out
```
Key pprof commands: `top`, `list`, `web`, `peek`

### Memory profiling
```bash
go test -memprofile=mem.out -bench=.
go tool pprof mem.out
```

### Trace
```bash
go test -trace=trace.out
go tool trace trace.out
```
Shows goroutine scheduling, GC events, syscall blocking.

### Runtime profiling
```go
import _ "net/http/pprof"
// go tool pprof http://localhost:6060/debug/pprof/profile
```

## Common Go optimizations

### Reduce allocations
- Use `sync.Pool` for reused objects
- Pre-allocate slices with `make([]T, 0, n)` when size is known
- Avoid `string([]byte)` conversions in hot paths
- Use `bytes.Buffer` instead of `fmt.Sprintf` in loops

### Efficient string handling
- Use `strings.Builder` for concatenation in loops
- Avoid `strings.Split` when you only need the first part
- Use `[]byte` for mutable text processing

### Map optimizations
- Pre-size maps when capacity is known
- Value types in maps avoid escape to heap

### Reduce goroutine overhead
- Use worker pools for bounded concurrency
- Use `semaphore.Weighted` for rate limiting
- Avoid goroutine-per-request for high-throughput paths

### I/O optimization
- Use `bufio` for buffered I/O
- Batch network calls (bulk APIs)
- Use connection pooling for databases
- Read files with `os.ReadFile` for small files, `bufio.Scanner` for large

### Struct layout
- Group fields by alignment (largest first)
- Use `bool` instead of `int` for flags
- Consider cache-line boundaries for hot structs

## Benchmarking
```go
func BenchmarkXxx(b *testing.B) {
    for i := 0; i < b.N; i++ {
        // code to benchmark
    }
}
```
- Use `b.ResetTimer()` after setup
- Use `b.ReportAllocs()` to track allocations
- Use `b.RunParallel()` for concurrent benchmarks
- Compare with `benchstat` tool

## Memory optimization
- Find allocations with escape analysis: `go build -gcflags=-m`
- Reduce pointer-to-value in hot paths (they escape to heap)
- Use value receivers when possible
- Unexported types can be value types safely

## Common anti-patterns
- Premature optimization (optimizing before profiling)
- Micro-optimizing cold paths
- Optimizing without tests (can't verify correctness)
- Overusing `sync.Pool` for simple objects
- Goroutine-per-request for high throughput

## When to stop optimizing
- When the bottleneck moves to I/O (network, disk)
- When further optimization adds complexity for diminishing returns
- When the code becomes hard to read and maintain
- When the performance meets the requirement

## Amdahl's law
Optimizing a part that takes 10% of total time can improve overall by at most 10%, even if that part becomes infinitely fast. Focus on the dominant cost.
