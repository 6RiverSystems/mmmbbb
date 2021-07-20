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

// this file is to be able to run the `main()` with coverage, to collect that
// data from the e2e smoke test script in CI

// see: https://www.elastic.co/blog/code-coverage-for-your-golang-system-tests
// see: https://www.confluent.io/blog/measure-go-code-coverage-with-bincover/

// we can get away with the lazy version of this for now because we don't have
// any CLI args to worry about, nor any expected panics or non-zero exit codes.

package main

import (
	"os"
	"testing"
)

func TestCoverMain(t *testing.T) {
	// only run this if _externally_ requested to do acceptance tests. note that
	// this is not the same as some other tests which temporarily set this env var
	// while they run.
	if os.Getenv("NODE_ENV") != "acceptance" {
		t.SkipNow()
	}
	main()
}
