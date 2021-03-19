package filter

type Filter = Condition

type Condition struct {
	Term *Term   `parser:"@@" json:",omitempty"`
	And  []*Term `parser:"( (\"AND\" @@ )+" json:",omitempty"`
	Or   []*Term `parser:"| (\"OR\" @@)+ )?" json:",omitempty"`
}

type Term struct {
	Not   bool             `parser:"@\"NOT\"?"`
	Basic *BasicExpression `parser:"( @@" json:",omitempty"`
	Sub   *Condition       `parser:"| \"(\" @@ \")\" )" json:",omitempty"`
}

type BasicExpression struct {
	Has       *HasAttribute          `parser:"@@" json:",omitempty"`
	Value     *HasAttributeValue     `parser:"| @@" json:",omitempty"`
	Predicate *HasAttributePredicate `parser:"| @@" json:",omitempty"`
}

type HasAttribute struct {
	Name string `parser:"\"attributes\" \":\" @(Ident|String)"`
}

type HasAttributeValue struct {
	Name  string            `parser:"\"attributes\" \".\" @(Ident|String)"`
	Op    AttributeOperator `parser:"@(\"=\" | \"!\" \"=\")"`
	Value string            `parser:"@String"`
}

type HasAttributePredicate struct {
	Predicate AttributePredicate `parser:"@(\"hasPrefix\")\"(\""`
	Name      string             `parser:"\"attributes\" \".\" @(Ident|String) \",\""`
	Value     string             `parser:"@String \")\""`
}

type BooleanOperator string

const (
	OpAND BooleanOperator = "AND"
	OpOR  BooleanOperator = "OR"
)

type AttributeOperator string

const (
	OpEqual    AttributeOperator = "="
	OpNotEqual AttributeOperator = "!="
)

type AttributePredicate string

const (
	PredicateHasPrefix AttributePredicate = "hasPrefix"
)
