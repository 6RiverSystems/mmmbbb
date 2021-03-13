// Package version contains an auto-generated version constant, sourced from
// semrel when built in CI, or else `git describe` for developer builds.
package version

//go:generate ./write-version.sh ../.version
//go:generate gofmt -s -w version.go
