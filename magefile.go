// Copyright (c) 2022 6 River Systems
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

//go:build mage
// +build mage

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"

	// tools this needs, to keep `go mod tidy` from deleting lines
	_ "github.com/golangci/golangci-lint/pkg/commands"
	"golang.org/x/sync/errgroup"
	_ "golang.org/x/tools/imports"
)

var Default = CompileAndTest
var Aliases = map[string]interface{}{
	"generate": GenerateDefault,
	"fmt":      Format,
	"compile":  CompileDefault,
	"lint":     LintDefault,
}

var goImportsFlags = []string{"-local", "github.com/6RiverSystems,go.6river.tech"}

//cSpell:ignore nomsgpack
var goBuildArgs = []string{"-tags", "nomsgpack"}
var goLintArgs = []string{"--build-tags", "nomsgpack"}

// always test with race and coverage, we'll run vet separately.
// unless CGO is disabled, and race is not available
var goTestArgs = []string{"-vet=off", "-cover", "-coverpkg=./..."}
var cmds = []string{"service"}
var goArches = []string{"amd64", "arm64"}

var generatedSimple = []string{
	"./ent/ent.go",
	"./oas/oas-types.go",
	"./version/version.go",
}
var generatedGrpc = []string{
	"./grpc/pubsub/pubsub_grpc.pb.go",
	"./grpc/pubsub/pubsub.pb.gw.go",
	"./grpc/pubsub/pubsub-types.go",
	"./grpc/pubsub.swagger.json",
	"./grpc/pubsub/schema_grpc.pb.go",
	"./grpc/pubsub/schema.pb.gw.go",
	"./grpc/pubsub/schema-types.go",
	"./grpc/schema.swagger.json",
}

//cspell:ignore Deps

func init() {
	// TODO: better way to detect CGO off?
	if os.Getenv("CGO_ENABLED") != "0" {
		goTestArgs = append(goTestArgs, "-race")
	}
}

func runAndCapture(cmd string, args ...string) (string, error) {
	outBuf := &bytes.Buffer{}
	var out io.Writer = outBuf
	if mg.Verbose() {
		out = io.MultiWriter(outBuf, os.Stdout)
	}
	if _, err := sh.Exec(nil, out, os.Stderr, cmd, args...); err != nil {
		return "", err
	}
	return outBuf.String(), nil
}

func splitWithoutBlanks(output string) []string {
	lines := strings.Split(output, "\n")
	ret := make([]string, 0, len(output))
	for _, l := range lines {
		if l != "" {
			ret = append(ret, l)
		}
	}
	return ret
}

func GenerateDefault(ctx context.Context) error {
	mg.CtxDeps(ctx, Generate{}.All)
	return nil
}

type Generate mg.Namespace

func (Generate) All(ctx context.Context) error {
	mg.CtxDeps(ctx, Generate{}.Ent, Generate{}.OAS, Generate{}.Version, Generate{}.Grpc)
	return nil
}

func (Generate) Force(ctx context.Context) error {
	if err := sh.Run("go", "generate", "-x", "./..."); err != nil {
		return err
	}
	mg.CtxDeps(ctx, FormatGenerated)
	return nil
}

func (Generate) Dir(ctx context.Context, dir string) error {
	fmt.Printf("Generate(%s)...\n", dir)
	if err := sh.Run("go", "generate", "-x", dir); err != nil {
		return err
	}
	mg.CtxDeps(ctx, mg.F(FormatDir, dir))
	return nil
}

func (Generate) Ent(ctx context.Context) error {
	if dirty, err := target.Path("./ent/ent.go", "./ent/generate.go", "go.mod", "go.sum"); err != nil {
		return err
	} else if !dirty {
		if dirty, err := target.Glob("./ent/ent.go", "./ent/schema/*.go"); err != nil {
			return err
		} else if !dirty {
			// clean
			return nil
		}
	}
	mg.CtxDeps(ctx, mg.F(Generate{}.Dir, "./ent"))
	return nil
}

func (Generate) OAS(ctx context.Context) error {
	if dirty, err := target.Path("./oas/oas-types.go", "./oas/generate.go", "./oas/openapi.yaml", "go.mod", "go.sum"); err != nil {
		return err
	} else if !dirty {
		return nil
	}
	mg.CtxDeps(ctx, mg.F(Generate{}.Dir, "./oas"))
	return nil
}

func (Generate) Version(ctx context.Context) error {
	if dirty, err := target.Path("./version/version.go", "./version/write-version.sh", ".git/index", ".git/refs/tags"); err != nil {
		return err
	} else if !dirty {
		if dirty, err := target.Path("./version/version.go", ".version"); err != nil {
			// .version might not exist
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else if !dirty {
			return nil
		}
	}
	mg.CtxDeps(ctx, mg.F(Generate{}.Dir, "./version"))
	return nil
}

func (Generate) DevVersion(ctx context.Context) error {
	out, err := sh.Output("git", "describe", "--tags", "--long", "--dirty", "--broken")
	if err != nil {
		return err
	}
	out = strings.TrimSpace(out)
	// trim the leading `v`
	out = out[1:]
	fmt.Printf("Generated(dev .version): %s\n", out)
	return os.WriteFile(".version", []byte(out+"\n"), 0644)
}

func (Generate) Grpc(ctx context.Context) error {
	dirty := false
	for _, out := range generatedGrpc {
		var err error
		if dirty, err = target.Path(out, "./grpc/generate.go"); err != nil {
			return err
		} else if dirty {
			break
		}
	}
	if !dirty {
		return nil
	}
	mg.CtxDeps(ctx, mg.F(Generate{}.Dir, "./grpc"))
	return nil
}

func Get(ctx context.Context) error {
	fmt.Println("Downloading dependencies...")
	if err := sh.Run("go", "mod", "download", "-x"); err != nil {
		return err
	}
	fmt.Println("Verifying dependencies...")
	if err := sh.Run("go", "mod", "verify"); err != nil {
		return err
	}
	return nil
}

func InstallProtobufTools(ctx context.Context) error {
	// CI needs apt-get update before packages can be installed, assume humans don't
	if os.Getenv("CI") != "" {
		if err := sh.Run("sudo", "apt-get", "update"); err != nil {
			return err
		}
	}
	// avoid sudo prompts if it's already installed
	needInstall := false
	if _, err := os.Stat("/usr/include/google/protobuf/empty.proto"); err != nil {
		needInstall = true
	} else if err := sh.Run("protoc", "--version"); err != nil {
		needInstall = true
	}
	if needInstall {
		if err := sh.Run("sudo", "apt-get", "-y", "install", "protobuf-compiler", "libprotobuf-dev"); err != nil {
			return err
		}
	}
	// versions of these packages will be picked up from go.mod
	if err := sh.Run("go", "install", "google.golang.org/protobuf/cmd/protoc-gen-go"); err != nil {
		return err
	}
	if err := sh.Run("go", "install", "google.golang.org/grpc/cmd/protoc-gen-go-grpc"); err != nil {
		return err
	}
	if err := sh.Run("go", "install", "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"); err != nil {
		return err
	}
	if err := sh.Run("go", "install", "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"); err != nil {
		return err
	}
	return nil
}

func InstallCITools(ctx context.Context) error {
	mg.CtxDeps(ctx, InstallProtobufTools)

	return sh.Run("go", "install", "gotest.tools/gotestsum")
}

/* TODO
tools:
	mkdir -p ./tools
	GOBIN=$(PWD)/tools go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen
	GOBIN=$(PWD)/tools go install entgo.io/ent/cmd/...
	GOBIN=$(PWD)/tools go install github.com/golangci/golangci-lint/cmd/golangci-lint
*/

// Format formats all the go source code
func Format(ctx context.Context) error {
	mg.CtxDeps(ctx, mg.F(FormatDir, "."))
	return nil
}

func FormatDir(ctx context.Context, dir string) error {
	fmt.Printf("Formatting(%s)...\n", dir)
	if err := sh.Run("gofmt", "-l", "-s", "-w", dir); err != nil {
		return err
	}
	goImportsArgs := []string{"run", "golang.org/x/tools/cmd/goimports", "-l", "-w"}
	goImportsArgs = append(goImportsArgs, goImportsFlags...)
	goImportsArgs = append(goImportsArgs, dir)
	if err := sh.Run("go", goImportsArgs...); err != nil {
		return err
	}
	return nil
}

// Format formats just the generated go source code
func FormatGenerated(ctx context.Context) error {
	fmt.Println("Formatting Generated...")
	out, err := sh.Output("git", "ls-files", "--exclude-standard", "--others", "--ignored", "-z")
	if err != nil {
		return err
	}
	var files []string
	for _, l := range strings.Split(out, "\x00") {
		if strings.HasSuffix(l, ".go") {
			files = append(files, l)
		}
	}
	if err := sh.Run("gofmt", append([]string{"-l", "-s", "-w", "."}, files...)...); err != nil {
		return err
	}
	goImportsArgs := []string{"run", "golang.org/x/tools/cmd/goimports", "-l", "-w"}
	goImportsArgs = append(goImportsArgs, goImportsFlags...)
	goImportsArgs = append(goImportsArgs, files...)
	if err := sh.Run("go", goImportsArgs...); err != nil {
		return err
	}
	return nil
}

type Lint mg.Namespace

// LintDefault runs all the lint:* targets
func LintDefault(ctx context.Context) error {
	mg.CtxDeps(ctx, GenerateDefault)
	mg.CtxDeps(ctx, Lint{}.Basic, Lint{}.Golangci)
	return nil
}

// Default runs all the lint:* targets
func (Lint) Default(ctx context.Context) error {
	return LintDefault(ctx)
}

func (Lint) Basic(ctx context.Context) error {
	mg.CtxDeps(ctx, Lint{}.Vet, Lint{}.Format, Lint{}.Imports, Lint{}.AddLicense)
	return nil
}

func (Lint) Ci(ctx context.Context) error {
	mg.CtxDeps(ctx, Lint{}.Basic, Lint{}.GolangciJUnit)
	return nil
}

func (Lint) Vet(ctx context.Context) error {
	fmt.Println("Linting(vet)...")
	return sh.RunV("go", "vet", "./...")
}

// Format checks that all Go source code follows formatting rules
func (Lint) Format(ctx context.Context) error {
	fmt.Println("Linting(gofmt)...")
	outStr, err := runAndCapture("gofmt", "-l", "-s", ".")
	if err != nil {
		return err
	}
	badFiles := splitWithoutBlanks(outStr)
	// TODO: ignore git-ignored files equivalent to piping through `fgrep -xvf <(
	// git ls-files --exclude-standard --others --ignored ) | grep .`
	if len(badFiles) != 0 {
		msg := &strings.Builder{}
		fmt.Fprintln(msg, "The following files need to be re-formatted:")
		for _, f := range badFiles {
			fmt.Fprintf(msg, "%s\n", f)
		}
		return errors.New(msg.String())
	}
	return nil
}

// Imports runs the goimports linting tool
func (Lint) Imports(ctx context.Context) error {
	fmt.Println("Linting(goimports)...")
	goImportsArgs := []string{"run"}
	if os.Getenv("VERBOSE") != "" {
		goImportsArgs = append(goImportsArgs, "-v")
	}
	goImportsArgs = append(goImportsArgs, "golang.org/x/tools/cmd/goimports", "-l")
	goImportsArgs = append(goImportsArgs, goImportsFlags...)
	goImportsArgs = append(goImportsArgs, ".")
	outStr, err := runAndCapture("go", goImportsArgs...)
	if err != nil {
		return err
	}
	badFiles := splitWithoutBlanks(outStr)
	// TODO: ignore git-ignored files equivalent to piping through `fgrep -xvf <(
	// git ls-files --exclude-standard --others --ignored ) | grep .`
	if len(badFiles) != 0 {
		msg := &strings.Builder{}
		fmt.Fprintln(msg, "The following files need to be re-formatted:")
		for _, f := range badFiles {
			fmt.Fprintf(msg, "%s\n", f)
		}
		return errors.New(msg.String())
	}
	return nil
}

// Golangci runs the golangci-lint tool
func (Lint) Golangci(ctx context.Context) error {
	fmt.Println("Linting(golangci)...")
	return Lint{}.golangci(ctx, false)
}

func (Lint) GolangciJUnit(ctx context.Context) error {
	fmt.Println("Linting(golangci)...")
	return Lint{}.golangci(ctx, true)
}

func (Lint) golangci(ctx context.Context, junit bool) error {
	args := []string{"run"}
	if os.Getenv("VERBOSE") != "" {
		args = append(args, "-v")
	}
	args = append(args, "github.com/golangci/golangci-lint/cmd/golangci-lint", "run")
	args = append(args, goLintArgs...)
	if os.Getenv("VERBOSE") != "" {
		args = append(args, "-v")
	}
	// CI reports being a 48 core machine or such, but we only get a couple cores
	if os.Getenv("CI") != "" && runtime.NumCPU() > 6 {
		args = append(args, "--concurrency", "6")
	}

	var err error
	outFile := os.Stdout
	if junit {
		args = append(args, "--out-format=junit-xml")
		resultsDir := os.Getenv("TEST_RESULTS")
		if resultsDir == "" {
			return fmt.Errorf("missing TEST_RESULTS env var")
		}
		outFileName := filepath.Join(resultsDir, "golangci-lint.xml")
		outFile, err = os.OpenFile(outFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer outFile.Close()
	}
	_, err = sh.Exec(map[string]string{}, outFile, os.Stderr, "go", args...)
	return err
}

// AddLicense runs the addlicense tool in check mode
func (Lint) AddLicense(ctx context.Context) error {
	return sh.Run(
		"go", "run", "github.com/google/addlicense",
		"-c", "6 River Systems",
		"-l", "mit",
		"-skip", "css",
		"-skip", "js",
		"-skip", "yml",
		"-skip", "html",
		"-ignore", "version/version.go",
		"-check",
		".",
	)
}

type Compile mg.Namespace

func CompileDefault(ctx context.Context) error {
	mg.CtxDeps(ctx, Compile{}.Code, Compile{}.Tests)
	return nil
}

func (Compile) Code(ctx context.Context) error {
	mg.CtxDeps(ctx, GenerateDefault)
	fmt.Println("Compiling(code)...")
	args := []string{"build", "-v"}
	args = append(args, goBuildArgs...)
	args = append(args, "./...")
	return sh.Run("go", args...)
}

func (Compile) Tests(ctx context.Context) error {
	mg.CtxDeps(ctx, GenerateDefault)
	fmt.Println("Compiling(tests)...")
	args := []string{"test"}
	args = append(args, goBuildArgs...)
	args = append(args, goTestArgs...)
	args = append(args, "-run=^$", "./...")
	return sh.Run("go", args...)
	// TODO: grep -v '\[no test' ; exit $${PIPESTATUS[0]}
}

func Test(ctx context.Context) error {
	mg.CtxDeps(ctx, LintDefault)
	mg.CtxDeps(ctx, TestGo)
	return nil
}

func TestGo(ctx context.Context) error {
	args := []string{"test"}
	args = append(args, goBuildArgs...)
	args = append(args, goTestArgs...)
	args = append(args, "-coverprofile=coverage.out", "./...")
	return sh.Run("go", args...)
}

func TestGoCISplit(ctx context.Context) error {
	// this target assumes some variables set on the make command line from the CI
	// run, and also that gotestsum is installed, which is not handled by this
	// makefile, but instead by the CI environment
	resultsDir := os.Getenv("TEST_RESULTS")
	if resultsDir == "" {
		return fmt.Errorf("missing TEST_RESULTS env var")
	}
	packageNames := strings.Split(os.Getenv("PACKAGE_NAMES"), " ")
	if len(packageNames) == 0 || packageNames[0] == "" {
		packageNames = []string{"./..."}
	}
	args := []string{"--format", "standard-quiet", "--junitfile", filepath.Join(resultsDir, "gotestsum-report.xml"), "--"}
	args = append(args, goBuildArgs...)
	args = append(args, goTestArgs...)
	args = append(args, "-coverprofile="+filepath.Join(resultsDir, "coverage.out"))
	args = append(args, packageNames...)
	return sh.Run("gotestsum", args...)
}

func TestSmoke(ctx context.Context, cmd, hostPort string) error {
	// TODO: this should just be a normal Go test

	resultsDir := os.Getenv("TEST_RESULTS")
	if resultsDir == "" {
		return fmt.Errorf("missing TEST_RESULTS env var")
	}

	eg, ctx := errgroup.WithContext(ctx)
	// start the test run in the background
	eg.Go(func() error {
		args := []string{
			"--format", "standard-quiet",
			"--junitfile", filepath.Join(resultsDir, "gotestsum-smoke-report-"+cmd+".xml"),
			"--",
		}
		args = append(args, goTestArgs...)
		args = append(args,
			"-coverprofile="+filepath.Join(resultsDir, "coverage-smoke-"+cmd+".out"),
			"-v",
			"-run", "TestCoverMain",
			"./"+filepath.Join("cmd", cmd),
		)
		// have to use normal exec so the context can terminate this
		cmd := exec.CommandContext(ctx, "gotestsum", args...)
		cmd.Env = append([]string{}, os.Environ()...)
		cmd.Env = append(cmd.Env, "NODE_ENV=acceptance")
		return cmd.Run()
	})
	eg.Go(func() error {
		// wait for the app to get running
		if mg.Verbose() {
			fmt.Printf("Waiting for app(%s) at %s...\n", cmd, hostPort)
		}
		for {
			conn, err := net.DialTimeout("tcp", hostPort, time.Minute)
			if err != nil {
				time.Sleep(50 * time.Millisecond)
			}
			if conn != nil {
				conn.Close()
				break
			}
		}
		// run a couple quick HTTP checks
		// TODO: these should be input specs too
		tryURL := func(m string, u *url.URL) error {
			if mg.Verbose() {
				fmt.Printf("Trying %s %s ...\n", m, u)
			}
			if req, err := http.NewRequestWithContext(ctx, m, u.String(), nil); err != nil {
				return err
			} else if resp, err := http.DefaultClient.Do(req); err != nil {
				return err
			} else {
				if resp.Body != nil {
					defer resp.Body.Close()
				}
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					return fmt.Errorf("failed %s %s: %d %s", m, u, resp.StatusCode, resp.Status)
				}
			}
			return nil
		}
		if err := tryURL(http.MethodGet, &url.URL{Scheme: "http", Host: hostPort, Path: "/"}); err != nil {
			return err
		}
		// TODO: poke some gRPC gateway endpoints
		if err := tryURL(http.MethodPost, &url.URL{Scheme: "http", Host: hostPort, Path: "/server/shutdown"}); err != nil {
			return err
		}
		return nil
	})
	return eg.Wait()
}

func CompileAndTest(ctx context.Context) error {
	mg.CtxDeps(ctx, CompileDefault, Test)
	return nil
}

// TODO: test-main-cover, smoke-test-curl-service

func CleanEnt(ctx context.Context) error {
	return sh.Run("git", "-C", "ent", "clean", "-fdX")
}

func Clean(ctx context.Context) error {
	mg.CtxDeps(ctx, CleanEnt)
	for _, f := range generatedSimple {
		if err := sh.Rm(f); err != nil {
			return err
		}
	}
	for _, f := range generatedGrpc {
		if err := sh.Rm(f); err != nil {
			return err
		}
	}
	for _, f := range []string{"bin", "coverage.out", "coverage.html", ".version"} {
		if err := sh.Rm(f); err != nil {
			return err
		}
	}
	if m, err := filepath.Glob("gonic.sqlite3*"); err != nil {
		return err
	} else {
		for _, f := range m {
			if err := sh.Rm(f); err != nil {
				return err
			}
		}
	}
	for _, d := range []string{"grpc/pubsub", "grpc/health"} {
		// just rmdir here, and ignore both doesn't exist and isn't empty
		if err := os.Remove(d); err != nil && !os.IsNotExist(err) && !os.IsExist(err) {
			return err
		}
	}

	return nil
}

// TODO: docker-dev-version

func ReleaseBinary(ctx context.Context, cmd, arch string) error {
	env := map[string]string{
		"GOARCH": arch,
	}
	if arch != runtime.GOARCH {
		env["CGO_ENABLED"] = "0"
	}
	args := []string{"build", "-v"}
	args = append(args, goBuildArgs...)
	args = append(args, "-o", filepath.Join("bin", cmd+"-"+arch), "./"+filepath.Join("cmd", cmd))
	return sh.RunWith(env, "go", args...)
}

func ReleaseBinaries(ctx context.Context) error {
	var fns []interface{}
	for _, cmd := range cmds {
		for _, arch := range goArches {
			fns = append(fns, mg.F(ReleaseBinary, cmd, arch))
		}
	}
	mg.CtxDeps(ctx, fns...)
	return nil
}

type Docker mg.Namespace

const multiArchBuilderName = "mmmbbb-multiarch"

func (Docker) MultiarchInitLocal(ctx context.Context) error {
	// this is for initializing a local machine for dev, CI needs a different flow
	// that's in the config there
	return sh.Run("docker", "buildx", "create", "--name", multiArchBuilderName, "--bootstrap")
}

func (Docker) MultiarchBuildAll(ctx context.Context) error {
	var fns []interface{}
	for _, cmd := range cmds {
		fns = append(fns, mg.F(Docker{}.MultiarchBuild, cmd))
	}
	// paralleling these just makes the output confusing
	mg.SerialCtxDeps(ctx, fns...)
	return nil
}

// MultiarchPushAll pushes all the multi-arch docker images. It actually has to
// rebuild them, so it relies on the docker build cache working well to be
// efficient.
func (Docker) MultiarchPushAll(ctx context.Context) error {
	var fns []interface{}
	for _, cmd := range cmds {
		fns = append(fns, mg.F(Docker{}.MultiarchPush, cmd))
	}
	// paralleling these just makes the output confusing
	mg.SerialCtxDeps(ctx, fns...)
	return nil
}

func (Docker) MultiarchBuild(ctx context.Context, cmd string) error {
	fmt.Printf("Docker-MultiArch(%s)...\n", cmd)
	return dockerRunMultiArch(ctx, cmd, false)
}
func (Docker) MultiarchPush(ctx context.Context, cmd string) error {
	fmt.Printf("Docker-MultiArchPush(%s)...\n", cmd)
	return dockerRunMultiArch(ctx, cmd, true)
}

func dockerRunMultiArch(ctx context.Context, cmd string, push bool) error {
	var args []string
	if os.Getenv("CI") != "" {
		args = append(args, "--context", "multiarch-context")
	}
	args = append(args, "buildx", "build", "--builder", multiArchBuilderName)
	var platforms []string
	for _, arch := range goArches {
		platforms = append(platforms, runtime.GOOS+"/"+arch)
	}
	args = append(args, "--platform", strings.Join(platforms, ","))
	var version string
	if content, err := os.ReadFile(".version"); err != nil {
		return err
	} else {
		version = strings.TrimSpace(string(content))
	}
	baseImage := "mmmbbb-" + cmd
	baseTag := baseImage + ":" + version
	if push {
		const gcrBase = "gcr.io/plasma-column-128721/"
		// push everything to gcr
		args = append(args, "-t", gcrBase+baseTag)
		if os.Getenv("CIRCLE_BRANCH") == "main" {
			// push latest tag on main
			args = append(args, "-t", gcrBase+baseImage+":latest")
			// also push to docker hub for builds on main
			if os.Getenv("DOCKERHUB_USER") != "" {
				mg.CtxDeps(ctx, Docker{}.HubLogin)
				args = append(args, "-t", "6river/"+baseTag)
				args = append(args, "-t", "6river/"+baseImage+":latest")
			}
		}
	} else {
		// base tag is not valid to push, but a useful local thing to use for the
		// only-build mode
		args = append(args, "-t", baseTag)
	}
	args = append(args, "--build-arg", "BINARYNAME="+cmd)
	if push {
		args = append(args, "--push")
	}
	args = append(args, ".")
	return sh.RunWithV(
		// TODO: not sure we need this
		map[string]string{"BINARYNAME": cmd},
		"docker", args...,
	)
}

func (Docker) HubLogin(ctx context.Context) error {
	user, password := os.Getenv("DOCKERHUB_USER"), os.Getenv("DOCKERHUB_PASSWORD")
	if user == "" || password == "" {
		return fmt.Errorf("missing DOCKERHUB_USER and/or DOCKERHUB_PASSWORD")
	}
	// have to use raw exec to set stdin
	cmd := exec.CommandContext(ctx, "docker", "login", "--username", user, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)
	if mg.Verbose() {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
