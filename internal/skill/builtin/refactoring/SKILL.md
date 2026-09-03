---
name: refactoring
description: Safe code refactoring techniques for Go: extract function, move method, rename, extract interface, simplify conditionals. Use when improving code structure without changing behavior.
origin: builtin
---

# Refactoring

## Purpose
Improve code structure, readability, and maintainability without changing external behavior.

## When to use
- Code is hard to read or understand
- Duplicated code across files/packages
- Functions too long or doing too many things
- Tight coupling between modules
- Preparing a codebase for a new feature

## Core principle
**Tests first.** Before refactoring, ensure the code has tests. If it doesn't, write characterization tests first. Refactoring without tests is not refactoring — it's guessing.

## Go-specific refactoring patterns

### Extract function
- Move a block of code into a named function
- Name describes what, not how
- Pass only what the function needs (not the whole context)

### Extract interface
- Define an interface at the consumer side
- Move from concrete to interface when multiple implementations exist
- Keep interfaces small (1-3 methods)

### Move method/type
- Relocate to the package that uses it most
- Update all importers
- Check dependency direction (don't create import cycles)

### Replace conditional with polymorphism
- Replace `switch type` with interface dispatch
- Each type implements the interface

### Simplify
- Replace nested if with early return (guard clauses)
- Replace magic numbers with named constants
- Remove dead code
- Consolidate duplicate conditional fragments

## Safe steps
1. Ensure tests pass before starting
2. Make one change at a time (atomic)
3. Run tests after each change
4. Commit after each successful step
5. If tests break, revert and understand why

## Common Go refactoring targets

### Long functions
- Extract logical steps into named helpers
- Replace comments with function names

### Deep nesting
- Guard clauses: early return for preconditions
- Extract nested logic to functions

### God objects (large structs)
- Split into cohesive smaller structs
- Use composition (embedding) instead of one mega-struct

### Duplicate code
- Extract to shared function
- If duplication is across types, extract interface

### Complex conditionals
- Named boolean variables for readability
- Extract condition to predicate function

## When NOT to refactor
- Right before a release (do it after)
- When you don't understand the code
- When there are no tests
- When the team is under time pressure

## Refactoring vs rewriting
- Refactoring: incremental, behavior-preserving, testable at each step
- Rewriting: full replacement, high risk, long time before it works
- Prefer refactoring; rewrite only when the architecture is fundamentally wrong

## Tools
- `go vet` — detects suspicious code
- `golangci-lint` — comprehensive linter
- `gofmt`/`goimports` — formatting consistency
- `deadcode` — find unused code
