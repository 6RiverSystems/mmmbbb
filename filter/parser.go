package filter

import (
	"github.com/alecthomas/participle/v2"
)

var DefaultParserOptions = []participle.Option{
	// participle.UseLookahead(1),
	// attribute values (or prefixes) need to be unquoted
	participle.Unquote("String"),
}

// func NewParser() (*participle.Parser, error) {
// 	return participle.Build(
// 		&Filter{},
// 		defaultParserOptions...,
// 	)
// }

var Parser = participle.MustBuild(
	&Filter{},
	DefaultParserOptions...,
)
