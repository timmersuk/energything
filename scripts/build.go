// Build both Linux targets without depending on a particular host shell.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	id := flag.String("build-id", "local", "public build identifier")
	goExe := flag.String("go", "go", "Go executable")
	flag.Parse()
	if strings.ContainsAny(*id, " \t\r\n") {
		fmt.Fprintln(os.Stderr, "build ID must not contain whitespace")
		os.Exit(1)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		dir := filepath.Join("bin", "linux", arch)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cmd := exec.Command(*goExe, "build", "-trimpath", "-ldflags", "-s -w -X main.BuildID="+*id, "-o", filepath.Join(dir, "energything"), "./cmd/server")
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GOOS=") && !strings.HasPrefix(v, "GOARCH=") && !strings.HasPrefix(v, "CGO_ENABLED=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
