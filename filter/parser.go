package filter

import (
	"github.com/alecthomas/participle/v2"
)

func NewParser() (*participle.Parser, error) {
	return participle.Build(
		&Filter{},
		participle.UseLookahead(3),
		// attribute values (or prefixes) need to be unquoted
		participle.Unquote("String"),
	)
}
