`/kg-scope → model a domain before writing code`

Act as a knowledge graph architect. I want to model a domain before writing any code.

Domain: [DESCRIBE IN 2 SENTENCES]
What I want to answer with it: [3 REAL QUESTIONS]

Return:
1. 8-12 entity types, each with the 3-5 attributes that matter and a note on what uniquely identifies an instance
2. 5-8 relation types as (subject type, predicate, object type), with cardinality
3. My 3 questions rewritten as traversals over those types
4. Anything my questions need that the schema cannot answer, and what's missing

Do not write code. If a question needs aggregation rather than traversal, say so — that's a database, not a graph.

If a project directory is available, persist the output as `kg-workspace/scope.yaml` so downstream commands (/kg-schema) can read it.
