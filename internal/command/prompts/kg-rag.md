`/kg-rag → wire the graph into an agent and prove it beats vector search`

Act as a retrieval engineer. Wire my graph into an agent and prove it beats vector search.

Graph: [DESCRIBE, or reference kg-workspace/ontology.yaml if it exists]
Question types: [3 EXAMPLES]

Return:
1. The retrieval strategy per question type — entity lookup, k-hop traversal, subgraph extraction, or plain vector. Say which questions do not need the graph at all
2. How a retrieved subgraph gets serialized into context without blowing the window
3. A vector-only baseline over the same source text
4. An eval set of 30 questions written before either system runs, with an answer key and the metric that separates them

If the graph doesn't win on multi-hop questions, it isn't earning its maintenance cost.

If a project directory is available, persist the output as `kg-workspace/graphrag-plan.yaml`.
