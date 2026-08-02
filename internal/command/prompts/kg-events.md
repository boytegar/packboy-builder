`/kg-events → model events as first-class nodes, not static edges`

Act as an event extraction engineer. I want a graph of things that happened, not things that are.

Domain and corpus: [DESCRIBE]

Return:
1. An event type schema: trigger, arguments and their roles, time anchor
2. The extraction prompt, one record per event, with argument spans
3. The edges between events — causal, temporal, conditional — and how to distinguish "reported as causing" from "merely co-occurred"
4. How to store this so a query can walk a chain backwards from an outcome

Keep event nodes separate from entity nodes. Never collapse a cause into an attribute.

If a project directory is available, persist the output as `kg-workspace/event-schema.yaml`.
