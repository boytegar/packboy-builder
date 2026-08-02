// Package graph implements the knowledge-graph half of graph engineering.
//
// It provides a schema-first knowledge graph store with provenance on every
// fact, plus the 9-stage pipeline machinery (ontology, extraction, fusion,
// evaluation, and GraphRAG serving). The task-graph half lives in the task
// sub-package.
//
// Design rules (from the graph-engineering skill):
//   - Schema first, always. Extraction without an ontology produces a word
//     cloud with arrows.
//   - Provenance on every fact: each node/edge stores source, extracted_at,
//     and confidence. Non-negotiable.
//   - Fuse before storing. Skipping fusion is the #1 cause of useless graphs.
//   - Incremental over big-bang: process a 10-document pilot through all 9
//     stages before scaling.
package graph
