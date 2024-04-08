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

package swaggerui

import (
	"encoding/json"
	"net/http"
)

const ConfigLoadingPath = "/oas-ui-config"

// setting anything except url(s) from the dynamic config may not work, see
// upstream bug https://github.com/swagger-api/swagger-ui/issues/4455.
var defaultConfig json.RawMessage = ([]byte)(`{
	"url": "../oas/openapi.yaml"
}`)

func DefaultConfigHandler() http.HandlerFunc {
	return CustomConfigHandler(func(config map[string]any) map[string]any { return config })
}

func CustomConfigHandler(
	customizer func(map[string]any) map[string]any,
) http.HandlerFunc {
	var config map[string]any
	var err error
	if err = json.Unmarshal(defaultConfig, &config); err != nil {
		// this should really be a compile error
		panic(err)
	}
	config = customizer(config)
	var msg []byte
	if msg, err = json.Marshal(config); err != nil {
		panic(err)
	}
	return func(writer http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		writer.Header().Add("Content-type", "application/json; charset=utf-8")
		if _, err := writer.Write(msg); err != nil {
			// this assumes we have recovery middleware
			panic(err)
		}
	}
}
