//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

// Default target to run when none is specified
// If not set, running mage will list available targets
// var Default = Build

const (
	binName     = "batch-tool"
	defaultArch = "amd64"

	osLinux   = "linux"
	osWindows = "windows"
	osDarwin  = "darwin"

	dirBuild   = "bin"
	dirPackage = "release"
)

var Aliases = map[string]interface{}{
	"build":   Build.All,
	"package": Package.All,
}

var targets = map[string][]string{
	osLinux:   []string{"amd64", "arm64"},
	osWindows: []string{"amd64"},
	osDarwin:  []string{"amd64", "arm64"},
}

var sources = []string{
	"magefile.go", "main.go", "go.mod", "go.sum", "call", "cmd", "config", "utils",
}

type Build mg.Namespace

// Build the executables for all supported platforms
func (Build) All() {
	// Declares target dependencies that will be run in parallel
	deps := make([]interface{}, 0)

	for targetOS, archList := range targets {
		for _, arch := range archList {
			deps = append(deps, mg.F(build, targetOS, arch))
		}
	}

	mg.Deps(deps...)
}

// Build the executable for Linux only
func (Build) Linux(arch string) error {
	return build(osLinux, arch)
}

// Build the executable for Windows only
func (Build) Windows(arch string) error {
	return build(osWindows, arch)
}

// Build the executable for MacOS only
func (Build) Darwin(arch string) error {
	return build(osDarwin, arch)
}

type Package mg.Namespace

// Package the executables for all supported platforms
func (Package) All() {
	// Declares target dependencies that will be run in parallel
	deps := make([]interface{}, 0)

	for targetOS, archList := range targets {
		for _, arch := range archList {
			deps = append(deps, mg.F(release, targetOS, arch))
		}
	}

	mg.Deps(deps...)
}

// Package the executable for Linux only
func (Package) Linux(arch string) error {
	return release(osLinux, arch)
}

// Package the executable for Windows only
func (Package) Windows(arch string) error {
	return release(osWindows, arch)
}

// Package the executable for MacOS only
func (Package) Darwin(arch string) error {
	return release(osDarwin, arch)
}

// Install the executable to your local system
func Install() error {
	if err := sh.RunV("go", "install"); err != nil {
		return err
	}
}

// Clean up after yourself
func Clean() error {
	fmt.Println("Cleaning build artifacts and release packages")

	var origErr error

	for _, path := range []string{dirBuild, dirPackage} {
		if err := sh.Rm(path); err != nil {
			origErr = err
		}
	}

	return origErr
}

// Helper function to call go build with the correct target OS and architecture
func build(targetOS, arch string) error {
	buildTarget := parseBuildTarget(targetOS, arch)
	env := map[string]string{
		"GOARCH": arch,
		"GOOS":   targetOS,
	}

	if updated, err := target.Path(buildTarget, sources...); !updated || err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(dirBuild, targetOS), 0755); err != nil {
		return err
	}

	fmt.Printf("Building %s/%s\n", targetOS, arch)

	return sh.RunWithV(env, "go", "build", "-o", buildTarget)
}

// Helper function to package a release for the given target OS and architecture
func release(targetOS, arch string) error {
	// package is dependent on build for the same OS/arch
	mg.Deps(mg.F(build, targetOS, arch))

	buildTarget := parseBuildTarget(targetOS, arch)
	packageTarget := parsePackageTarget(targetOS, arch)

	// check if built binaries have been updated since the last package execution
	if updated, err := target.Path(packageTarget, buildTarget); !updated || err != nil {
		return err
	}

	if err := os.MkdirAll(dirPackage, 0755); err != nil {
		return err
	}

	fmt.Printf("Packaging %s/%s\n", targetOS, arch)

	if targetOS == osWindows {
		return sh.Run("zip", "-j", packageTarget, buildTarget)
	}

	// While much more verbose, this could be fleshed out with a go-native implementation that would be cross-platform compatible.
	// Relevant standard library packages would be archive and compress.
	return sh.RunV("tar", "-caf", packageTarget, "-C", filepath.Dir(buildTarget), filepath.Base(buildTarget))
}

func parseBuildTarget(targetOS, arch string) string {
	filename := fmt.Sprintf("%s-%s", binName, arch)
	if targetOS == osWindows {
		filename += ".exe"
	}

	return filepath.Join(dirBuild, targetOS, filename)
}

func parsePackageTarget(targetOS, arch string) string {
	filename := fmt.Sprintf("%s-%s-%s", binName, targetOS, arch)
	if targetOS == osWindows {
		filename += ".zip"
	} else {
		filename += ".txz"
	}

	return filepath.Join(dirPackage, filename)
}

// findGoBinDir returns the path used by go install based on the documented logic here: https://go.dev/ref/mod#go-install
func findGoBinDir() string {
	if path, ok := os.LookupEnv("GOBIN"); ok {
		return path
	}

	if path, ok := os.LookupEnv("GOPATH"); ok {
		return filepath.Join(path, "bin")
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, "go", "bin")
}
