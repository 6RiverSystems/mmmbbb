#!/bin/sh

# TODO: replace this with something less terrible
v=v0.0.0-development
if [ -n "$1" -a -f "$1" ]; then
	v="$(cat "$1")"
else
	v="$(git describe --tags --long --dirty --broken | cut -c 2-)"
fi

(
	echo "package version"
	echo "const SemrelVersion = \"${v}\""
) >version.go
