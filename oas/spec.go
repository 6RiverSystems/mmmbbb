package oas

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
)

// LoadSpec parses the spec resource, as a stand-in for the spec generation from
// oapi-codegen, to avoid double-embedding the YAML in the binary.
func LoadSpec() (*openapi3.Swagger, error) {
	yamlBytes, err := OpenAPIFS.ReadFile("openapi.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "error loading Swagger")
	}
	swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData(yamlBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error loading Swagger")
	}
	return swagger, nil
}

func MustLoadSpec() *openapi3.Swagger {
	swagger, err := LoadSpec()
	if err != nil {
		panic(err)
	}
	return swagger
}
