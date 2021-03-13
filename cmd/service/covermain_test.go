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
