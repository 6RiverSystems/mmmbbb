// this isn't part of the actual code build
// +build generate

package ent

import (
	_ "entgo.io/ent/entc/gen"
	_ "github.com/olekukonko/tablewriter"
)

//go:generate go run entgo.io/ent/cmd/ent generate --feature entql ./schema
