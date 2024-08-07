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

//go:build xyzzy
// +build xyzzy

// this file just exists so that our linting-related dependencies won't get
// tidied out of go.mod. Some of these things aren't actually importable
// packages, so we have to put a build tag on this so it's not compiled

package internal

import (
	_ "github.com/google/addlicense"
	_ "golang.org/x/tools/imports"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "gotest.tools/gotestsum/cmd"

	_ "github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	// needed for golangci-lint
	_ "github.com/quasilyte/go-ruleguard/dsl"

	// prevents dependabot from moving this module dependency to indirect
	_ "github.com/golang/protobuf/ptypes/empty"
)
