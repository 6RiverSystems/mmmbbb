#!/bin/bash

# Copyright (c) 2022 6 River Systems
#
# Permission is hereby granted, free of charge, to any person obtaining a copy of
# this software and associated documentation files (the "Software"), to deal in
# the Software without restriction, including without limitation the rights to
# use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
# the Software, and to permit persons to whom the Software is furnished to do so,
# subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
# FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
# COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
# IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
# CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

# TODO: replace this and the makefile with a magefile

set -xeuo pipefail

BINARY_NAMES=(service)
ARCHS=(amd64 arm64)
BUILDARGS=(-tags nomsgpack)

nativearch=$(dpkg --print-architecture)

for bin in "${BINARY_NAMES[@]}" ; do
	for arch in "${ARCHS[@]}" ; do
		if [ "$arch" = "$nativearch" ]; then
			CGO_ENABLED=1
		else
			CGO_ENABLED=0
		fi
		CGO_ENABLED=${CGO_ENABLED} GOARCH=${arch} \
			go build -v "${BUILDARGS[@]}" -ldflags "-s -w" \
			-o "bin/${bin}-${arch}" \
			./cmd/${bin}
	done
done

if ! docker buildx inspect mmmbbb-multiarch ; then
	docker buildx create --name mmmbbb-multiarch --bootstrap
fi

platforms=()
for arch in "${ARCHS[@]}" ; do
	platforms+=("linux/${arch}")
done
platforms="${platforms[*]}"
platforms="${platforms// /,}"
builderargs=(--builder mmmbbb-multiarch)
ctxargs=()
if [ "${CI}" ]; then
	ctxargs+=(--context multiarch-context)
fi
for bin in "${BINARY_NAMES}" ; do
	basetag="mmmbbb-${bin}:$(<.version)"
	tagargs=(-t "$basetag")
	if [ "${CIRCLE_BRANCH:-}" = "main" ]; then
		tagargs+=(-t "gcr.io/plasma-column-128721/${basetag}")
	fi
	if [ "${DOCKERHUB_USER:-}" ]; then
		docker login --username "$DOCKERHUB_USER" --password-stdin <<< "$DOCKERHUB_PASSWORD"
		tagargs+=(-t "6river/${basetag}")
	fi
	BINARYNAME=bin docker \
		"${ctxargs[@]}" \
		buildx build \
		"${builderargs[@]}" \
		--platform "${platforms}" \
		"${tagargs[@]}" \
		--build-arg "BINARYNAME=${bin}" \
		.
done
