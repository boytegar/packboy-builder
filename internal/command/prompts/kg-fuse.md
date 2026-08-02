`/kg-fuse → deduplicate and merge entities across sources`

Act as an entity resolution engineer. My graph has duplicates.

Entity type and volume: [e.g. 40k company records]
Available fields: [LIST THEM]

Return:
1. A blocking strategy so I'm not doing n-squared comparisons, with the expected reduction
2. The match function: which fields, which similarity measure, which weights, which threshold
3. A review band — the score range where a human decides instead of the machine
4. A merge policy: on conflict, which source wins, and what survives as an alias rather than being discarded
5. 10 hard cases from my field list where the naive approach fails

Merges must be reversible. Tell me what to log so I can undo one.

If a project directory is available, persist the output as `kg-workspace/fusion-plan.yaml`.
