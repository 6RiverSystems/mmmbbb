// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package filter

type Filter = Condition

type Condition struct {
	Term *Term   `parser:"@@" json:",omitempty"`
	And  []*Term `parser:"( (\"AND\" @@ )+" json:",omitempty"`
	Or   []*Term `parser:"| (\"OR\" @@)+ )?" json:",omitempty"`
}

type Term struct {
	Not   bool             `parser:"@(\"NOT\"|\"-\")?"`
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
