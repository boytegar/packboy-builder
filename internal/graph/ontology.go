package graph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ── Ontology ────────────────────────────────────────────────────

// Ontology is the schema that governs the knowledge graph. It defines entity
// types, relation types (with domain/range constraints), and attributes.
// Modeled after OWL/RDFS: every relation has a precise verb name
// (ACQUIRED, not RELATED_TO), and types always queried together are merged.
type Ontology struct {
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	EntityTypes []EntityType `json:"entity_types" yaml:"entity_types"`
	Relations   []Relation   `json:"relations" yaml:"relations"`
	Version     string       `json:"version,omitempty" yaml:"version,omitempty"`
}

// EntityType defines a class of entities in the domain.
type EntityType struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	SubClassOf  string            `json:"sub_class_of,omitempty" yaml:"sub_class_of,omitempty"`
	Attributes  []Attribute       `json:"attributes,omitempty" yaml:"attributes,omitempty"`
	Identifies  string            `json:"identifies,omitempty" yaml:"identifies,omitempty"` // what uniquely identifies an instance
	Props       map[string]string `json:"props,omitempty" yaml:"props,omitempty"`
}

// Attribute is a property of an entity type.
type Attribute struct {
	Name       string `json:"name" yaml:"name"`
	Type       string `json:"type" yaml:"type"` // string, int, float, bool, datetime
	Required   bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Functional bool   `json:"functional,omitempty" yaml:"functional,omitempty"` // at most one value
}

// Relation defines a typed edge between entity types. Every relation gets a
// precise verb name (ACQUIRED, not RELATED_TO). Domain/range constrain which
// entity types may appear at each endpoint — this one validation step removes
// most hallucinated structure.
type Relation struct {
	Name        string `json:"name" yaml:"name"` // verb, e.g. "ACQUIRED"
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Domain      string `json:"domain" yaml:"domain"`                               // subject entity type
	Range       string `json:"range" yaml:"range"`                                 // object entity type
	Cardinality string `json:"cardinality,omitempty" yaml:"cardinality,omitempty"` // 1:1, 1:N, N:M
	Functional  bool   `json:"functional,omitempty" yaml:"functional,omitempty"`
	Inverse     string `json:"inverse,omitempty" yaml:"inverse,omitempty"` // inverse relation name
}

// Validate checks the ontology for internal consistency. Returns the first
// error encountered. Checks: relation domain/range reference declared entity
// types, subclass targets exist, no duplicate names.
func (o *Ontology) Validate() error {
	if o == nil {
		return errors.New("ontology is nil")
	}
	if o.Name == "" {
		return errors.New("ontology name is required")
	}
	if len(o.EntityTypes) == 0 {
		return errors.New("ontology must declare at least one entity type")
	}

	// Build entity type name set.
	typeSet := make(map[string]struct{}, len(o.EntityTypes))
	for _, et := range o.EntityTypes {
		if et.Name == "" {
			return errors.New("entity type with empty name")
		}
		if _, dup := typeSet[et.Name]; dup {
			return fmt.Errorf("duplicate entity type: %s", et.Name)
		}
		typeSet[et.Name] = struct{}{}
	}

	// Validate subclass targets.
	for _, et := range o.EntityTypes {
		if et.SubClassOf != "" {
			if _, ok := typeSet[et.SubClassOf]; !ok {
				return fmt.Errorf("entity type %s: sub_class_of %s not declared", et.Name, et.SubClassOf)
			}
		}
	}

	// Validate relations.
	relSet := make(map[string]struct{}, len(o.Relations))
	for _, r := range o.Relations {
		if r.Name == "" {
			return errors.New("relation with empty name")
		}
		if _, dup := relSet[r.Name]; dup {
			return fmt.Errorf("duplicate relation: %s", r.Name)
		}
		relSet[r.Name] = struct{}{}

		if r.Domain == "" || r.Range == "" {
			return fmt.Errorf("relation %s: domain and range are required", r.Name)
		}
		if _, ok := typeSet[r.Domain]; !ok {
			return fmt.Errorf("relation %s: domain %s not declared", r.Name, r.Domain)
		}
		if _, ok := typeSet[r.Range]; !ok {
			return fmt.Errorf("relation %s: range %s not declared", r.Name, r.Range)
		}
	}

	return nil
}

// EntityType returns the entity type declaration by name, or nil.
func (o *Ontology) EntityType(name string) *EntityType {
	for i := range o.EntityTypes {
		if o.EntityTypes[i].Name == name {
			return &o.EntityTypes[i]
		}
	}
	return nil
}

// Relation returns the relation declaration by name, or nil.
func (o *Ontology) Relation(name string) *Relation {
	for i := range o.Relations {
		if o.Relations[i].Name == name {
			return &o.Relations[i]
		}
	}
	return nil
}

// RelationValidFor checks whether a relation can connect the given subject
// and object entity types. This is the domain/range check that rejects
// hallucinated edges. Allows subclass substitution via IsA.
func (o *Ontology) RelationValidFor(relName, subjectType, objectType string) bool {
	r := o.Relation(relName)
	if r == nil {
		return false
	}
	// Exact match or subclass substitution.
	return o.IsA(subjectType, r.Domain) && o.IsA(objectType, r.Range)
}

// SubClassesOf returns all entity types that are (transitively) subclasses of
// the given parent type.
func (o *Ontology) SubClassesOf(parent string) []string {
	var result []string
	for _, et := range o.EntityTypes {
		if et.SubClassOf == parent {
			result = append(result, et.Name)
			result = append(result, o.SubClassesOf(et.Name)...)
		}
	}
	return result
}

// IsA returns true if child is parent or a transitive subclass of parent.
func (o *Ontology) IsA(child, parent string) bool {
	if child == parent {
		return true
	}
	for _, et := range o.EntityTypes {
		if et.Name == child {
			if et.SubClassOf == "" {
				return false
			}
			return o.IsA(et.SubClassOf, parent)
		}
	}
	return false
}

// Summary returns a human-readable summary of the ontology for display.
func (o *Ontology) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ontology: %s (%d entity types, %d relations)\n", o.Name, len(o.EntityTypes), len(o.Relations))
	if o.Description != "" {
		fmt.Fprintf(&b, "  %s\n", o.Description)
	}
	b.WriteString("\nEntity Types:\n")
	names := make([]string, 0, len(o.EntityTypes))
	for _, et := range o.EntityTypes {
		names = append(names, et.Name)
	}
	sort.Strings(names)
	for _, n := range names {
		et := o.EntityType(n)
		suffix := ""
		if et.SubClassOf != "" {
			suffix = " (subClassOf " + et.SubClassOf + ")"
		}
		fmt.Fprintf(&b, "  - %s%s [%d attributes]\n", et.Name, suffix, len(et.Attributes))
	}
	b.WriteString("\nRelations:\n")
	relNames := make([]string, 0, len(o.Relations))
	for _, r := range o.Relations {
		relNames = append(relNames, r.Name)
	}
	sort.Strings(relNames)
	for _, n := range relNames {
		r := o.Relation(n)
		fmt.Fprintf(&b, "  - %s: %s → %s", r.Name, r.Domain, r.Range)
		if r.Cardinality != "" {
			fmt.Fprintf(&b, " [%s]", r.Cardinality)
		}
		if r.Inverse != "" {
			fmt.Fprintf(&b, " (inverse: %s)", r.Inverse)
		}
		b.WriteString("\n")
	}
	return b.String()
}
