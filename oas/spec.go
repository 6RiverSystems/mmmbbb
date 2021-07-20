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

package oas

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// LoadSpec parses the spec resource, as a stand-in for the spec generation from
// oapi-codegen, to avoid double-embedding the YAML in the binary.
func LoadSpec() (*openapi3.T, error) {
	yamlBytes, err := OpenAPIFS.ReadFile("openapi.yaml")
	if err != nil {
		return nil, fmt.Errorf("error loading Swagger: %w", err)
	}
	swagger, err := openapi3.NewLoader().LoadFromData(yamlBytes)
	if err != nil {
		return nil, fmt.Errorf("error loading Swagger: %w", err)
	}
	return swagger, nil
}

func MustLoadSpec() *openapi3.T {
	swagger, err := LoadSpec()
	if err != nil {
		panic(err)
	}
	return swagger
}
