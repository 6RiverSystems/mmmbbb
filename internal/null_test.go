//+build xyzzy

// this file just exists so that our linting-related dependencies won't get
// tidied out of go.mod. Some of these things aren't actually importable
// packages, so we have to put a build tag on this so it's not compiled

package internal

import (
	_ "github.com/golangci/golangci-lint/pkg/commands"
	_ "github.com/google/addlicense"
	_ "golang.org/x/tools/imports"
	_ "gotest.tools/gotestsum/cmd"
)
