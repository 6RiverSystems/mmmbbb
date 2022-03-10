#!/bin/bash

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
if [ "${CI}" ]; then
	builderargs+=(--context mmmbbb-multiarch)
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
	BINARYNAME=bin docker buildx build \
		"${builderargs[@]}" \
		--platform "${platforms}" \
		"${tagargs[@]}" \
		--build-arg "BINARYNAME=${bin}" \
		.
done
