---
name: database-design
description: Database schema design, migration strategy, indexing, and query optimization for SQL databases. Use when designing database schemas, writing migrations, or optimizing queries.
origin: builtin
---

# Database Design

## When to use
- Designing a new database schema
- Writing database migrations
- Optimizing slow queries
- Planning indexing strategy

## Schema design principles
- **Normalize first, denormalize when needed**: start with 3NF, denormalize only when query performance demands it
- **Use appropriate data types**: don't use `TEXT` for a 2-char code; use `VARCHAR(2)`
- **Foreign keys are mandatory**: they enforce referential integrity
- **NOT NULL by default**: only allow NULL when the value is genuinely optional
- **Constraints over application logic**: `UNIQUE`, `CHECK` constraints catch bugs the app can't
- **Timestamps**: use `TIMESTAMP WITH TIME ZONE` (UTC storage), not `TIMESTAMP`
- **Primary keys**: prefer `BIGSERIAL`/`UUID` over composite keys for simplicity

## Indexing strategy
- Index foreign keys (they're not auto-indexed in PostgreSQL)
- Index columns used in `WHERE`, `JOIN`, `ORDER BY` clauses
- Use composite indexes when queries filter on multiple columns
- Order composite index by selectivity (most selective first)
- Use partial indexes for sparse conditions: `WHERE deleted_at IS NULL`
- Use `EXPLAIN ANALYZE` to verify index usage
- Don't over-index: every index slows down writes

## Migration patterns
- **Forward-only**: prefer forward migrations, never destructive
- **Additive first**: add column (nullable), deploy app, backfill, add NOT NULL
- **Rename in phases**: add new column → dual-write → migrate reads → drop old
- **Never drop in the same migration that removes the code**: wait a release
- **Test migrations on production-sized data**: migrations that work on dev can time out on prod

## Migration safety
- Add columns with defaults in separate transactions
- Create indexes `CONCURRENTLY` (PostgreSQL) to avoid table locks
- Avoid `ALTER TABLE` that rewrites the table on large tables
- Backfill data in batches, not in one transaction

## Query optimization
- Use `EXPLAIN ANALYZE` — not just `EXPLAIN`
- Look for: sequential scans on large tables, nested loops with many iterations
- N+1 query problem: use `Preload`/`Joins`/eager loading
- Avoid `SELECT *` — select only needed columns
- Use `LIMIT` with `ORDER BY` for pagination (or cursor pagination)
- Use `IN` with reasonable batch sizes, not huge arrays

## Common anti-patterns
- EAV (Entity-Attribute-Value) anti-pattern
- Using UUIDs as clustered index (insertion hotspot on GUID v4)
- Storing JSON in a column and querying it frequently (use JSONB with GIN index)
- Polymorphic associations (no real foreign key)
- Missing indexes on foreign keys
- Over-normalization (more joins than necessary)

## PostgreSQL specifics
- Use `JSONB` not `JSON` for queryable JSON
- Use `UUID` with `gen_random_uuid()` for random UUIDs
- `pg_stat_statements` for query performance analysis
- `pg_repack` for online index rebuilds
- Connection pooling: `pgxpool` or PgBouncer

## SQLite specifics
- WAL mode for concurrent reads while writing
- `PRAGMA foreign_keys = ON` (off by default!)
- Good for development, embedded, and small-scale production
- Migrations via raw SQL or `goose`/`golang-migrate`
