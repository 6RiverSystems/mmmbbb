package filter

type Filter = Condition

type Condition struct {
	Or []*OrTerm `parser:"@@ (\"OR\" @@)*" json:",omitempty"`
}

type OrTerm struct {
	And []*AndTerm `parser:"@@ (\"AND\" @@)*" json:",omitempty"`
}

type AndTerm struct {
	Not   bool             `parser:"@\"NOT\"?"`
	Basic *BasicExpression `parser:"( @@" json:",omitempty"`
	Sub   *Condition       `parser:"| \"(\" @@ \")\" )" json:",omitempty"`
}

type BasicExpression struct {
	HasAttribute          *HasAttribute          `parser:"@@" json:",omitempty"`
	HasAttributePredicate *HasAttributePredicate `parser:"| @@" json:",omitempty"`
}

type HasAttribute struct {
	Name    string   `parser:"\"attributes\" \":\" @(Ident|String)"`
	OpValue *OpValue `parser:"@@?" json:",omitempty"`
}

type OpValue struct {
	Op    AttributeOperator `parser:"@(\"=\" | \"!\" \"=\")"`
	Value string            `parser:"@String"`
}

type HasAttributePredicate struct {
	Predicate AttributePredicate `parser:"@(\"hasPrefix\")\"(\""`
	Name      string             `parser:"\"attributes\" \":\" @(Ident|String) \",\""`
	Value     string             `parser:"@String \")\""`
}

type AttributeOperator string

const (
	OpEqual    = "="
	OpNotEqual = "!="
)

type AttributePredicate string

const (
	PredicateHasPrefix = "hasPrefix"
)
