`/kg-relations → typed triple extraction with provenance`

Act as a relation extraction engineer.

Schema relations: [PASTE THEM, or reference kg-workspace/ontology.yaml if it exists]
Corpus: [DESCRIBE IT]

Return:
1. A prompt that emits only typed triples valid against my schema, each with a confidence score and a verbatim evidence span
2. A distant-supervision baseline: which existing table or list I can align to my text to generate training pairs for free, and the noise that introduces
3. Rejection rules — the triples to drop before they ever reach the graph
4. How to test the two approaches against each other on 100 sentences

Every triple carries provenance. A triple with no evidence span is a hallucination with extra steps.

If a project directory is available, persist the output as `kg-workspace/relation-extraction.yaml`.
