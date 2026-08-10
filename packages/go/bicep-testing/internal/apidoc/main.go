// Command apidoc updates or verifies the checked-in Go public API documentation.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type apiTarget struct {
	packagePath  string
	baselinePath string
	displayName  string
}

var targets = []apiTarget{
	{packagePath: ".", baselinePath: "api/go/biceptesting.txt", displayName: "biceptesting"},
	{
		packagePath:  "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client",
		baselinePath: "api/go/bicep-rpc-client.txt",
		displayName:  "biceprpcclient",
	},
}

func main() {
	update := flag.Bool("update", false, "update checked-in API files")
	check := flag.Bool("check", false, "check API files without changing them")
	flag.Parse()
	if *update == *check {
		fmt.Fprintln(os.Stderr, "specify exactly one of --update or --check")
		os.Exit(2)
	}

	moduleRoot, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}
	repositoryRoot := filepath.Clean(filepath.Join(moduleRoot, "..", "..", ".."))
	for _, target := range targets {
		generated, err := generate(moduleRoot, target.packagePath)
		if err != nil {
			fatal(err)
		}
		baselinePath := filepath.Join(repositoryRoot, target.baselinePath)
		if *update {
			if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
				fatal(err)
			}
			if err := os.WriteFile(baselinePath, generated, 0o644); err != nil {
				fatal(err)
			}
			fmt.Printf("Updated %s\n", target.baselinePath)
			continue
		}

		baseline, err := os.ReadFile(baselinePath)
		if err != nil {
			fatal(fmt.Errorf("read %s: %w", target.baselinePath, err))
		}
		if !bytes.Equal(generated, normalize(baseline)) {
			fmt.Fprintf(os.Stderr, "Go public API has changed for %s. Review it and run go generate ./...\n", target.displayName)
			os.Exit(1)
		}
		fmt.Printf("Go public API is up to date for %s.\n", target.displayName)
	}
}

func findModuleRoot() (string, error) {
	goExecutable := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goExecutable += ".exe"
	}
	command := exec.Command(goExecutable, "env", "GOMOD")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("locate Go module: %w", err)
	}
	goModPath := strings.TrimSpace(string(output))
	if goModPath == "" || goModPath == os.DevNull {
		return "", fmt.Errorf("current directory is not inside a Go module")
	}
	return filepath.Dir(goModPath), nil
}

func generate(moduleRoot, packagePath string) ([]byte, error) {
	goExecutable := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goExecutable += ".exe"
	}
	command := exec.Command(goExecutable, "doc", "-all", packagePath)
	command.Dir = moduleRoot
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("generate API for %s: %s", packagePath, strings.TrimSpace(string(output)))
	}
	return normalize(output), nil
}

func normalize(value []byte) []byte {
	normalized := strings.ReplaceAll(string(value), "\r\n", "\n")
	return []byte(strings.TrimSpace(normalized) + "\n")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
