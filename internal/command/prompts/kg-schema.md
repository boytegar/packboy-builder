`/kg-schema → turn a scope draft into a real ontology`

Act as an ontology engineer. Turn this draft schema into a real ontology.

Draft: [PASTE YOUR /kg-scope OUTPUT, or reference kg-workspace/scope.yaml if it exists]

Return:
1. A class hierarchy with explicit subclass relations, no more than 3 levels deep
2. Every property with domain, range, and whether it's functional or inverse-functional
3. Turtle serialization I can load straight into Protégé
4. Every modeling decision where you chose between two defensible options, and why

Reuse schema.org or an existing vocabulary for anything generic — only mint new IRIs for what's specific to my domain. Flag anything you modeled as a class that should have been an instance.

If a project directory is available, persist the output as `kg-workspace/ontology.yaml` (machine-readable) and `kg-workspace/ontology.ttl` (Turtle) so downstream commands (/kg-extract) can read it.
