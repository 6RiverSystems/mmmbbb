package filter

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

type AsFilter interface {
	AsFilter() (string, error)
}

func (e *Condition) AsFilter() (string, error) {
	if len(e.Or) == 0 {
		return "", errors.New("unpopulated Condition")
	}
	b := strings.Builder{}
	for i, ee := range e.Or {
		if i > 0 {
			b.WriteString(" OR ")
		}
		s, err := ee.AsFilter()
		if err != nil {
			return "", errors.Wrap(err, "error in OrTerm")
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

func (e *OrTerm) AsFilter() (string, error) {
	if len(e.And) == 0 {
		return "", errors.New("unpopulated OrTerm")
	}
	b := strings.Builder{}
	for i, ee := range e.And {
		if i > 0 {
			b.WriteString(" AND ")
		}
		s, err := ee.AsFilter()
		if err != nil {
			return "", errors.Wrap(err, "error in AndTerm")
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

func (e *AndTerm) AsFilter() (result string, err error) {
	switch {
	case e.Basic != nil:
		result, err = e.Basic.AsFilter()
	case e.Sub != nil:
		result, err = e.Sub.AsFilter()
		if err == nil {
			result = "(" + result + ")"
		}
	default:
		return "", errors.New("unpopulated AndTerm")
	}
	if e.Not && err == nil {
		result = "NOT " + result
	}
	return
}

func (e *BasicExpression) AsFilter() (string, error) {
	switch {
	case e.HasAttribute != nil:
		return e.HasAttribute.AsFilter()
	case e.HasAttributePredicate != nil:
		return e.HasAttributePredicate.AsFilter()
	default:
		return "", errors.New("unpopulated BasicExpression")
	}
}

func (e *HasAttribute) AsFilter() (string, error) {
	var b strings.Builder
	writeAttr(&b, e.Name)
	if e.OpValue != nil {
		b.WriteString(string(e.OpValue.Op))
		b.WriteString(strconv.Quote(e.OpValue.Value))
	}
	return b.String(), nil
}

func (e *HasAttributePredicate) AsFilter() (string, error) {
	var b strings.Builder
	b.WriteString(string(e.Predicate))
	b.WriteRune('(')
	writeAttr(&b, e.Name)
	b.WriteRune(',')
	b.WriteString(strconv.Quote(e.Value))
	b.WriteRune(')')
	return b.String(), nil
}

func writeAttr(b *strings.Builder, name string) {
	b.WriteString("attributes:")
	isIdent := true
	for i, ch := range name {
		// stolen from scanner.Scanner.isIdentRune
		if !(ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0) {
			isIdent = false
			break
		}
	}
	if isIdent {
		b.WriteString(name)
	} else {
		b.WriteString(strconv.Quote(name))
	}

}
