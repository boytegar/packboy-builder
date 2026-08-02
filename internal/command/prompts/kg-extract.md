`/kg-extract → design the extraction pipeline before building it`

Act as an extraction engineer. Design the pipeline before I build it.

Sources: [LIST THEM — e.g. 400 PDFs, a Postgres table, scraped HTML]
Target schema: [PASTE /kg-schema OUTPUT, or reference kg-workspace/ontology.yaml if it exists]

Return:
1. Split my sources into structured / semi-structured / unstructured, and the method for each — the first two should not need a model
2. For the unstructured set: the prompt, the output JSON schema, the chunking strategy
3. The 5 failure modes most likely for this specific data, with a detection check for each
4. A 50-document hand-check protocol: what I sample, what I record, what number tells me to stop tuning

Do not propose fine-tuning until the prompted baseline has a measured error rate.

If a project directory is available, persist the output as `kg-workspace/extraction-plan.yaml` so downstream commands can read it.
