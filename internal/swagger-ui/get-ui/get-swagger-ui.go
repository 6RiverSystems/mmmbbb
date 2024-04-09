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

// this isn't part of the actual code build
//go:build generate
// +build generate

// it needs to be in a separate directory to keep vscode and gopls from getting
// angry about package name mismatches

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
)

// update this by running: `npm view swagger-ui-dist@latest --json dist`
var swaggerUiDistInfo = `{
  "integrity": "sha512-vQ+Pe73xt7vMVbX40L6nHu4sDmNCM6A+eMVJPGvKrifHQ4LO3smH0jCiiefKzsVl7OlOcVEnrZ9IFzYwElfMkA==",
  "shasum": "64c96e90b6a352e7b20a55b73b91fc0e0bed4f0a",
  "tarball": "https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-5.11.3.tgz",
  "fileCount": 24,
  "unpackedSize": 10297440,
  "signatures": [
    {
      "keyid": "SHA256:jl3bwswu80PjjokCgh0o2w5c2U4LhQAE57gj9cz1kzA",
      "sig": "MEQCIFeZEvtocUK6K8u4u1aq2Zu0mOj6YV7ZrxKCHhPaBlHlAiAsmJ1VErPuWFvPAGs7i55ZcLvxCY3uwAOhhkbECop1Vg=="
    }
  ]
}`

func main() {
	var distInfo map[string]interface{}
	if err := json.Unmarshal([]byte(swaggerUiDistInfo), &distInfo); err != nil {
		panic(err)
	}
	resp, err := http.Get(distInfo["tarball"].(string))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		panic("Failed to get swagger-ui-dist tarball")
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	actualSha1 := sha1.Sum(buf)
	correctSha1, err := hex.DecodeString(distInfo["shasum"].(string))
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(actualSha1[:], correctSha1) {
		panic(fmt.Errorf("swagger-ui-dist sha1 mismatch, want %s got %s", distInfo["shasum"], hex.EncodeToString(actualSha1[:])))
	}
	zReader, err := gzip.NewReader(bytes.NewReader(buf))
	if err != nil {
		panic(err)
	}
	err = os.Mkdir("ui", 0o777) // rely on umask
	if err != nil && !errors.Is(err, os.ErrExist) {
		panic(err)
	}
	tReader := tar.NewReader(zReader)
	for {
		h, err := tReader.Next()
		if h == nil || err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		if h.Typeflag != tar.TypeReg {
			continue
		}

		// we only want certain files
		switch h.Name {
		case "package/index.html",
			"package/favicon-16x16.png",
			"package/favicon-32x32.png",
			"package/swagger-ui-bundle.js",
			// "package/swagger-ui-bundle.js.map",
			"package/swagger-ui-standalone-preset.js",
			// "package/swagger-ui-standalone-preset.js.map",
			"package/swagger-ui.css",
			"package/index.css":
			// write this into the ui dir
			fileData, err := io.ReadAll(tReader)
			if err != nil {
				panic(err)
			}
			if err = os.WriteFile(path.Join("ui", path.Base(h.Name)), fileData, 0o666); err != nil {
				panic(err)
			}
		case "package/swagger-initializer.js":
			// retain the original file for comparisons, but always keep our custom file
			fileData, err := io.ReadAll(tReader)
			if err != nil {
				panic(err)
			}
			if err = os.WriteFile(path.Join("ui", "swagger-initializer.orig.js"), fileData, 0o666); err != nil {
				panic(err)
			}
			if err = os.WriteFile(path.Join("ui", "swagger-initializer.js"), []byte(customInitializerContent), 0o666); err != nil {
				panic(err)
			}
		}
	}
}

const customInitializerContent = `
window.onload = function() {
	window.ui = SwaggerUIBundle({
		configUrl: '../oas-ui-config',
		dom_id: "#swagger-ui",
		deepLinking: true,
		layout: "StandaloneLayout",
		displayOperationId: true,
		docExpansion: "none",
		queryConfigEnabled: true,
		showCommonExtensions: true,
		tryItOutEnabled: true,
		withCredentials: true,
		validatorUrl: null,
		presets: [
			SwaggerUIBundle.presets.apis,
			SwaggerUIStandalonePreset
		],
		plugins: [
			SwaggerUIBundle.plugins.DownloadUrl
		],
		layout: "StandaloneLayout"
	});
};
`
