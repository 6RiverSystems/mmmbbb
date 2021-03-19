package filter

import (
	"github.com/alecthomas/participle/v2"
)

var DefaultParserOptions = []participle.Option{
	participle.UseLookahead(50),
	// attribute values (or prefixes) need to be unquoted
	participle.Unquote("String"),
}

var Parser = participle.MustBuild(
	&Filter{},
	DefaultParserOptions...,
)
