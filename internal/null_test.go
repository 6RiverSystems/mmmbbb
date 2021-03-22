package internal

// this file just exists so that our linting-related dependencies won't get
// tidied out of go.mod

import (
	_ "github.com/golangci/golangci-lint/pkg/commands"
	_ "golang.org/x/tools/imports"
	_ "gotest.tools/gotestsum/cmd"
)
