GENERATE_SIMPLE:=\
	./ent/ent.go \
	./oas/oas-types.go \
	./version/version.go \
	$(NULL)
GENERATE_GRPC:=\
	./grpc/pubsub/pubsub_grpc.pb.go \
	./grpc/pubsub/pubsub.pb.gw.go \
	./grpc/pubsub/pubsub-types.go \
	./grpc/pubsub.swagger.json \
	./grpc/pubsub/schema_grpc.pb.go \
	./grpc/pubsub/schema.pb.gw.go \
	./grpc/pubsub/schema-types.go \
	./grpc/schema.swagger.json \
	./grpc/health/health_grpc.pb.go \
	./grpc/health/health.pb.go \
	./grpc/health.swagger.json \
	$(NULL)
GENERATE_SPECIAL:=\
	$(NULL)
GENERATED_FILES:=\
	$(GENERATE_SIMPLE) \
	$(GENERATE_GRPC) \
	$(GENERATE_SPECIAL) \
	$(NULL)

ifneq ($(CIRCLE_PROJECT_REPONAME),)
REPONAME:=$(CIRCLE_PROJECT_REPONAME)
else
REPONAME:=$(notdir $(CURDIR))
endif

BINARY_NAMES:=\
	service \
	$(NULL)
BINARIES:=$(patsubst %,bin/%,$(BINARY_NAMES))
BINARIES_DOCKER:=$(patsubst %,docker-%,$(BINARY_NAMES))
BINARIES_DOCKER_PUSH:=$(patsubst %,docker-push-%,$(BINARY_NAMES))

# always test with race and coverage, we'll run vet separately.
TESTARGS:=-vet=off -race -cover -coverpkg=./...

GOIMPORTSARGS:=-local github.com/6RiverSystems,go.6river.tech

BUILDARGS:=-tags nomsgpack
LINTARGS:=$(patsubst -tags,--build-tags,$(BUILDARGS))

# default `make` invocation is to run the full generate/build/lint/test sequence
default: compile-code test
.PHONY: default

generate: $(GENERATED_FILES)
.PHONY: generate

$(GENERATE_SIMPLE): %.go:
	go generate -x ./$(dir $@)
	gofmt -l -s -w ./$(dir $@)
	go run golang.org/x/tools/cmd/goimports -l -w $(GOIMPORTSARGS) ./$(dir $@)

$(GENERATE_GRPC) &: ./grpc/generate.go
	go generate -x ./$(dir $^)
	gofmt -l -s -w ./$(dir $^)
	go run golang.org/x/tools/cmd/goimports -l -w $(GOIMPORTSARGS) ./$(dir $^)

# specific additional dependencies (these will share the generation rule)
# third party generator runs depend on go.mod/sum as that may change the
# generator version
./ent/ent.go: ./ent/generate.go $(wildcard ./ent/schema/*.go) go.mod go.sum
./oas/oas-types.go: ./oas/generate.go ./oas/openapi.yaml go.mod go.sum

./version/version.go: ./version/generate.go ./version/write-version.sh .git/index .git/refs/tags $(wildcard .version)

# special rules
./common/swagger-ui/ui/swagger-ui-bundle.js: ./common/swagger-ui/generate.go ./common/swagger-ui/get-ui/get-swagger-ui.go
	go generate -x ./common/swagger-ui
	gofmt -l -s -w ./common/swagger-ui
	go run golang.org/x/tools/cmd/goimports -l -w $(GOIMPORTSARGS) ./common/swagger-ui

# this requires at least go 1.16.5 to avoid mucking up go.mod/go.sum
get:
	go mod download -x
	go mod verify

install-ci-tools: install-tools-protobuf
# tools only needed in CI
	go install gotest.tools/gotestsum
tools:
	mkdir -p ./tools
	GOBIN=$(PWD)/tools go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen
	GOBIN=$(PWD)/tools go install entgo.io/ent/cmd/...
	GOBIN=$(PWD)/tools go install github.com/golangci/golangci-lint/cmd/golangci-lint
.PHONY: get install-ci-tools tools

fmt:
	gofmt -l -s -w .
	go run golang.org/x/tools/cmd/goimports -l -w $(GOIMPORTSARGS) .
# format just the generated files
fmt-generated: $(GENERATED_FILES)
	git ls-files --exclude-standard --others --ignored -z | grep -z '\.go$$' | xargs -0 gofmt -l -s -w
	git ls-files --exclude-standard --others --ignored -z | grep -z '\.go$$' | xargs -0 go run golang.org/x/tools/cmd/goimports -l -w $(GOIMPORTSARGS)
.PHONY: fmt fmt-generated

# <() construct requires bash
lint : SHELL=/bin/bash
lint:
# use inverted grep exit code to both print results and fail if there are any
# fgrep -xvf... is used to exclude exact matches from the list of git ignored files
	! gofmt -l -s . | fgrep -xvf <( git ls-files --exclude-standard --others --ignored ) | grep .
	! go run golang.org/x/tools/cmd/goimports -l $(GOIMPORTSARGS) . | fgrep -xvf <( git ls-files --exclude-standard --others --ignored ) | grep .
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run $(LINTARGS)
	go run github.com/google/addlicense -c '6 River Systems' -l mit -skip css -skip js -skip yml -skip html -ignore version/version.go -check .
.PHONY: lint

compile: compile-code compile-tests
compile-code: generate
	go build -v $(BUILDARGS) ./...
# this weird hack makes go compile the tests but not run them. basically this
# seeds the build cache and gives us any compile errors. unforunately it also
# prints out test-like output, so we have to hide that with some grep.
# PIPESTATUS requires bash
compile-tests : SHELL = /bin/bash
compile-tests: generate
	go test $(BUILDARGS) $(TESTARGS) -run='^$$' ./... | grep -v '\[no test' ; exit $${PIPESTATUS[0]}
.PHONY: compile compile-code compile-tests

# paranoid: always test with the race detector
test: lint vet test-go
vet:
	go vet $(BUILDARGS) ./...
test-go:
	go test $(BUILDARGS) $(TESTARGS) -coverprofile=coverage.out ./...
test-go-ci-split:
# this target assumes some variables set on the make command line from the CI
# run, and also that gotestsum is installed, which is not handled by this
# makefile, but instead by the CI environment
	gotestsum --format standard-quiet --junitfile $(TEST_RESULTS)/gotestsum-report.xml -- $(BUILDARGS) $(TESTARGS) -coverprofile=${TEST_RESULTS}/coverage.out $(PACKAGE_NAMES)
.PHONY: test vet test-go test-go-ci-split
$(patsubst %,test-main-cover-%,$(BINARY_NAMES)): test-main-cover-%: $(TEST_RESULTS)
	NODE_ENV=acceptance gotestsum --format standard-quiet --junitfile $(TEST_RESULTS)/gotestsum-smoke-report-$*.xml -- $(TESTARGS) -coverprofile=${TEST_RESULTS}/coverage-smoke-$*.out -v -run TestCoverMain ./cmd/$*/
smoke-test-curl-service:
	curl --fail -X GET http://localhost:3001 && echo
# TODO: poke some gRPC gateway endpoints
	curl --fail -X POST http://localhost:3001/server/shutdown && echo
.PHONY: test-main-cover-% $(patsubst %,test-main-cover-%,$(BINARY_NAMES)) $(patsubst %,smoke-test-curl-%,$(BINARY_NAMES))

binaries: $(BINARIES)
.PHONY: binaries

$(BINARIES): bin/%: ./cmd/%/main.go compile-code
	go build -v $(BUILDARGS) -o $@ ./cmd/$*

clean-ent:
# -X says to only remove ignored files, not untracked ones
	git -C ent clean -fdX
clean: clean-ent
	rm -rf $(GENERATED_FILES) bin/ coverage.out coverage.html gonic.sqlite3* .version
	-rmdir --ignore-fail-on-non-empty grpc/pubsub/ grpc/health/
.PHONY: clean clean-ent

docker: binaries $(BINARIES_DOCKER)
# NOTE that the .version file is not automatically created by this Makefile,
# only by the CI process or human action. However this target for manually
# creating it from git data, like version.go is created, is provided for testing
# purposes.
docker-dev-version:
	git describe --tags --long --dirty --broken | cut -c 2- | tee .version
$(BINARIES_DOCKER): docker-%: bin/% Dockerfile .dockerignore $(wildcard .docker-deps/*) .version
# TODO: store the stripped binary in a different location so it's still useful for debugging
	strip $<
	BINARYNAME=$* docker build -t $(REPONAME)-$*:$(file <.version) --build-arg BINARYNAME=$* .
docker-push: $(BINARIES_DOCKER_PUSH)
# TODO: integrate this better with ci_tool.sh
GCRNAME:=gcr.io/plasma-column-128721
HUBNAME:=6river
$(BINARIES_DOCKER_PUSH): docker-push-%: .version
	docker tag $(REPONAME)-$*:$(file <.version) $(GCRNAME)/$(REPONAME)-$*:$(file <.version)
	docker push $(GCRNAME)/$(REPONAME)-$*:$(file <.version)
ifeq ($(CIRCLE_BRANCH),main)
	docker tag $(REPONAME)-$*:$(file <.version) $(GCRNAME)/$(REPONAME)-$*:latest
	docker push $(GCRNAME)/$(REPONAME)-$*:latest
# only publish to public docker hub account on `main`, and if we have the perms
# to do so
ifneq ($(DOCKERHUB_USER),)
	echo "$$DOCKERHUB_PASSWORD" | docker login --username "$$DOCKERHUB_USER" --password-stdin
	docker tag $(REPONAME)-$*:$(file <.version) $(HUBNAME)/$(REPONAME)-$*:$(file <.version)
	docker tag $(REPONAME)-$*:$(file <.version) $(HUBNAME)/$(REPONAME)-$*:latest
	docker push $(HUBNAME)/$(REPONAME)-$*:$(file <.version)
	docker push $(HUBNAME)/$(REPONAME)-$*:latest
endif
endif
.PHONY: docker docker-dev-version $(BINARIES_DOCKER)

install-tools-protobuf:
# CI needs apt-get update before packages can be installed, assume humans don't
	if [ "$$CI" ]; then sudo apt-get update ; fi
# avoid sudo prompts if it's already installed
	if ! protoc --version || [ ! -f /usr/include/google/protobuf/empty.proto ] ; then \
		sudo apt-get -y install protobuf-compiler libprotobuf-dev ; \
	fi
# versions of these packages will be picked up from go.mod
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
.PHONY: install-tools-protobuf
