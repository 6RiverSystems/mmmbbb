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
