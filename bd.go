package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	Binaries []*Binary `json:"binaries"`
	BinDir   string    `json:"binDir"`
}

type Binary struct {
	Package string `json:"package"`
	Version string `json:"version"`
	Name    string `json:"name"`
}

const configFileName = "bd.json"

func main() {
	if len(os.Args) < 2 {
		printUsageAndExit()
	}

	config, err := loadConfig()
	if err != nil {
		die(fmt.Sprintf("Failed to load %s: %v", configFileName, err))
	}

	binDir, err := filepath.Abs(config.BinDir)
	if err != nil {
		die(fmt.Sprintf("Failed to resolve BinDir: %v", err))
	}

	switch os.Args[1] {
	case "install":
		if err := installBinaries(config, binDir); err != nil {
			die(fmt.Sprintf("Failed to install binaries: %v", err))
		}
	case "exec":
		if len(os.Args) < 3 {
			printUsageAndExit()
		}
		execBinary(config, binDir, os.Args[2], os.Args[3:])
	default:
		printUsageAndExit()
	}
}

func printUsageAndExit() {
	fmt.Println("Usage: bd <install|exec>")
	os.Exit(1)
}

func die(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func loadConfig() (*Config, error) {
	file, err := os.ReadFile(configFileName)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	if config.BinDir == "" {
		config.BinDir = "bin"
	}

	for _, bin := range config.Binaries {
		if err := normalizeBinary(bin); err != nil {
			return nil, fmt.Errorf("normalize %v: %v", bin, err)
		}
	}

	return &config, nil
}

func normalizeBinary(bin *Binary) error {
	parts := strings.Split(bin.Package, "@")
	if len(parts) > 1 {
		bin.Package, bin.Version = parts[0], parts[1]
	}
	if bin.Version == "" {
		bin.Version = "latest"
	}
	if bin.Name == "" {
		bin.Name = filepath.Base(bin.Package)
	}
	return nil
}

func installBinaries(config *Config, binDir string) error {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("create binDir: %w", err)
	}

	for _, bin := range config.Binaries {
		if err := installBinary(*bin, binDir); err != nil {
			return fmt.Errorf("install binary %s: %w", bin.Name, err)
		}
	}

	fmt.Printf("\nAll binaries installed in %s\n", binDir)
	return nil
}

func installBinary(bin Binary, binDir string) error {
	finalPath := filepath.Join(binDir, buildBinName(bin.Name, bin.Version))
	symlinkPath := filepath.Join(binDir, buildSymlinkName(bin.Name))

	if _, err := os.Stat(finalPath); err == nil && bin.Version != "latest" {
		fmt.Printf("Already installed: %s %s\n", bin.Name, bin.Version)
		return symlinkBinary(finalPath, symlinkPath)
	}

	tempDir, err := os.MkdirTemp("", "bd-build")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("go", "install", fmt.Sprintf("%s@%s", bin.Package, bin.Version))
	cmd.Env = append(os.Environ(), "GOBIN="+tempDir)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install %s: %w", bin.Package, err)
	}

	files, err := os.ReadDir(tempDir)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("find built binary in %s", tempDir)
	}

	binaryPath := filepath.Join(tempDir, files[0].Name())
	if err := os.Rename(binaryPath, finalPath); err != nil {
		return fmt.Errorf("move binary to final path: %w", err)
	}

	if err := symlinkBinary(finalPath, symlinkPath); err != nil {
		return fmt.Errorf("symlink binary: %w", err)
	}

	fmt.Printf("Installed: %s %s\n", bin.Name, bin.Version)

	return nil
}

func symlinkBinary(target, link string) error {
	if _, err := os.Stat(link); err == nil {
		_ = os.Remove(link)
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}
	return nil
}

func execBinary(config *Config, binDir, name string, args []string) {
	for _, bin := range config.Binaries {
		if bin.Name == name {
			binPath := filepath.Join(binDir, buildBinName(bin.Name, bin.Version))
			if _, err := os.Stat(binPath); os.IsNotExist(err) {
				die(fmt.Sprintf("Binary '%s' is not installed. Run 'bd install' first.", name))
			}
			execCmd(binPath, args)
			return
		}
	}
	die(fmt.Sprintf("Binary '%s' not found in bd.json", name))
}

func execCmd(binPath string, args []string) {
	cmd := exec.Command(binPath, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Run(); err != nil {
		die(fmt.Sprintf("Failed to execute %s: %v", binPath, err))
	}
}

func buildSymlinkName(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func buildBinName(name, version string) string {
	binName := fmt.Sprintf("%s-%s", name, version)
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	return binName
}
