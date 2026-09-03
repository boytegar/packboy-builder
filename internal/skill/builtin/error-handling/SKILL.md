---
name: error-handling
description: Error handling patterns for Go: wrapping, sentinel errors, custom error types, error inspection, and error-driven control flow. Use when designing error handling strategy or improving error messages.
origin: builtin
---

# Error Handling

## When to use
- Designing an error handling strategy for a Go package
- Deciding between sentinel errors, custom types, and error wrapping
- Improving error messages for debuggability
- Building error-driven APIs

## Go error philosophy
Errors are values, not exceptions. Handle them explicitly at every call site. Don't panic for expected conditions.

## Core patterns

### Sentinel errors
```go
var ErrNotFound = errors.New("not found")
// Usage: if errors.Is(err, ErrNotFound) { ... }
```
Use for: specific conditions callers need to match on. Keep them exported and documented.

### Custom error types
```go
type ValidationError struct {
    Field   string
    Message string
}
func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
// Usage: var ve *ValidationError; if errors.As(err, &ve) { ... }
```
Use for: errors carrying structured data callers need to inspect.

### Error wrapping
```go
if err := db.Save(user); err != nil {
    return fmt.Errorf("save user %s: %w", user.ID, err)
}
```
Always wrap with context. The `%w` verb preserves the original error for `errors.Is`/`errors.As`.

### errors.Is vs errors.As
- `errors.Is(err, target)` — check if err matches a sentinel (or wraps it)
- `errors.As(err, &target)` — extract structured data from a custom error type
- Both traverseance the wrap chain

## Best practices
- **Wrap at every boundary**: `fmt.Errorf("doing X: %w", err)`
- **Don't wrap twice**: if the inner error already has context, don't repeat it
- **Add the "what" not the "how"**: "save user" not "execute SQL INSERT"
- **Return early**: don't nest in if/else; use guard clauses
- **Don't log and return**: either handle it (log + recover) or return it (let the caller decide)

## Anti-patterns
- `if err != nil { return err }` without wrapping (loses context)
- `if err != nil { panic(err) }` for expected conditions
- `if err != nil { log.Fatal(err) }` in library code
- `return errors.New("something went wrong")` — vague, not actionable
- Ignoring errors with `_ = someFunc()`
- Wrapping with `fmt.Errorf("%v", err)` — breaks error chain (use `%w`)

## Error-driven control flow
Use `errors.Is` for control flow (retry, fallback, user-facing message). Use `errors.As` for structured inspection. Don't use string matching on `err.Error()`.

## Panic vs error
- **Error**: expected, recoverable condition (file not found, invalid input)
- **Panic**: unexpected, unrecoverable condition (invariant violation, nil dereference)
- In libraries: return errors, never panic
- In `main`: can `log.Fatal` for unrecoverable startup errors

## HTTP error mapping
- Map domain errors to HTTP status codes at the handler boundary
- `ErrNotFound → 404`, `ErrInvalidInput → 400`, `ErrConflict → 409`
- Don't leak internal error messages to API responses
