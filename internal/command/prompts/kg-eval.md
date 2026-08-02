`/kg-eval → skeptical review of the knowledge graph`

Act as a skeptical reviewer of my knowledge graph.

What I built: [DESCRIBE]
Numbers I'm about to claim: [PASTE THEM]

Return:
1. Precision and recall at the triple level — how to sample and estimate them with a stated confidence interval, not a vibe
2. Where my test set leaks into my training or prompt-development set
3. If I'm reporting link prediction: whether the filtered setting was used, and what a trivial baseline would score
4. The three claims a reviewer attacks first, and the experiment that defends each

Assume my numbers are inflated until the sampling method proves otherwise.

If a project directory is available, persist the output as `kg-workspace/eval-report.yaml`.
