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
	return oneOf(attrs, e.HasAttribute, e.HasAttributePredicate)
}

func (e *Condition) Nil() bool { return e == nil }
func (e *Condition) Evaluate(attrs map[string]string) (bool, error) {
	i := -1
	return anyOf(attrs, func() (Evaluator, bool) {
		i++
		if i >= len(e.Or) {
			return nil, false
		}
		return e.Or[i], true
	})
}

func (e *OrTerm) Evaluate(attrs map[string]string) (bool, error) {
	i := -1
	return allOf(attrs, func() (Evaluator, bool) {
		i++
		if i >= len(e.And) {
			return nil, false
		}
		return e.And[i], true
	})
}

func (e *AndTerm) Evaluate(attrs map[string]string) (bool, error) {
	result, err := oneOf(attrs, e.Basic, e.Sub)
	result = result != e.Not // XOR
	return result, err
}

func (e *HasAttribute) Nil() bool { return e == nil }
func (e *HasAttribute) Evaluate(attrs map[string]string) (bool, error) {
	v, ok := attrs[e.Name]
	if !ok {
		return false, nil
	}
	if e.OpValue == nil {
		return true, nil
	}
	switch e.OpValue.Op {
	case OpEqual:
		return v == e.OpValue.Value, nil
	case OpNotEqual:
		return v != e.OpValue.Value, nil
	default:
		return false, fmt.Errorf("invalid Op '%s'", e.OpValue.Op)
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

func oneOf(attrs map[string]string, options ...NillableEvaluator) (result bool, err error) {
	found := false
	for _, o := range options {
		if o == nil || o.Nil() {
			continue
		}
		if found {
			return result, errors.New("oneOf requires exactly one non-nil option")
		}
		found = true
		if result, err = o.Evaluate(attrs); err != nil {
			return
		}
	}
	if !found {
		err = errors.New("oneOf requires a non-nil option")
	}
	return
}

func allOf(attrs map[string]string, iterator func() (Evaluator, bool)) (result bool, err error) {
	found := false
	for {
		o, ok := iterator()
		if !ok {
			break
		}
		found = true
		if result, err = o.Evaluate(attrs); err != nil || !result {
			return
		}
	}
	if !found {
		err = errors.New("allOf requires a non-nil option")
	}
	return
}

func anyOf(attrs map[string]string, iterator func() (Evaluator, bool)) (result bool, err error) {
	found := false
	for {
		o, ok := iterator()
		if !ok {
			break
		}
		found = true
		if result, err = o.Evaluate(attrs); err != nil || result {
			return
		}
	}
	if !found {
		err = errors.New("anyOf requires a non-nil option")
	}
	return
}
