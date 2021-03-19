package filter

import (
	"errors"
	"fmt"
	"strings"
)

type Evaluator interface {
	Evaluate(attrs map[string]string) (bool, error)
}
type NillableEvaluator interface {
	Evaluator
	Nil() bool
}

func (e *BasicExpression) Nil() bool { return e == nil }
func (e *BasicExpression) Evaluate(attrs map[string]string) (bool, error) {
	switch {
	case e.Has != nil:
		return e.Has.Evaluate(attrs)
	case e.Value != nil:
		return e.Value.Evaluate(attrs)
	case e.Predicate != nil:
		return e.Predicate.Evaluate(attrs)
	default:
		return false, errors.New("unpopulated BasicExpression")
	}
}

func (e *Condition) Nil() bool { return e == nil }
func (e *Condition) Evaluate(attrs map[string]string) (result bool, err error) {
	result, err = e.Term.Evaluate(attrs)
	if err != nil {
		return
	}
	switch {
	case e.And != nil:
		if result {
			result, err = andTerms(attrs, e.And)
		}
	case e.Or != nil:
		if !result {
			result, err = orTerms(attrs, e.Or)
		}
	}
	return
}

func (e *Term) Nil() bool { return e == nil }
func (e *Term) Evaluate(attrs map[string]string) (result bool, err error) {
	switch {
	case e.Basic != nil:
		result, err = e.Basic.Evaluate(attrs)
	case e.Sub != nil:
		result, err = e.Sub.Evaluate(attrs)
	default:
		return false, errors.New("unpopulated Term")
	}
	result = result != e.Not // XOR
	return result, err
}

func (e *HasAttribute) Nil() bool { return e == nil }
func (e *HasAttribute) Evaluate(attrs map[string]string) (bool, error) {
	_, ok := attrs[e.Name]
	return ok, nil
}

func (e *HasAttributeValue) Nil() bool { return e == nil }
func (e *HasAttributeValue) Evaluate(attrs map[string]string) (bool, error) {
	v, ok := attrs[e.Name]
	if !ok {
		return false, nil
	}
	switch e.Op {
	case OpEqual:
		return v == e.Value, nil
	case OpNotEqual:
		return v != e.Value, nil
	default:
		return false, fmt.Errorf("invalid Op '%s'", e.Op)
	}
}

func (e *HasAttributePredicate) Nil() bool { return e == nil }
func (e *HasAttributePredicate) Evaluate(attrs map[string]string) (bool, error) {
	v, ok := attrs[e.Name]
	if !ok {
		return false, nil
	}
	switch e.Predicate {
	case PredicateHasPrefix:
		return strings.HasPrefix(v, e.Value), nil
	default:
		return false, fmt.Errorf("invalid predicate '%s'", e.Predicate)
	}
}

func andTerms(attrs map[string]string, terms []*Term) (result bool, err error) {
	if len(terms) == 0 {
		return false, errors.New("AND requires a non-empty term list")
	}
	for _, t := range terms {
		if result, err = t.Evaluate(attrs); err != nil || !result {
			return
		}
	}
	return
}

func orTerms(attrs map[string]string, terms []*Term) (result bool, err error) {
	if len(terms) == 0 {
		return false, errors.New("OR requires a non-empty term list")
	}
	for _, t := range terms {
		if result, err = t.Evaluate(attrs); err != nil || result {
			return
		}
	}
	return
}
