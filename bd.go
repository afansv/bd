package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		fmt.Println("Usage: bd <install|exec>")
		os.Exit(1)
	}

	command := os.Args[1]

	config, err := loadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to load bd.yaml: %v\n", err)
		os.Exit(1)
	}

	binDir, err := filepath.Abs(config.BinDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to resolve BinDir: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "install":
		if err := installBinaries(config, binDir); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to install binaries: %v\n", err)
			os.Exit(1)
		}
	case "exec":
		if len(os.Args) < 3 {
			fmt.Println("Usage: bd exec <name> [...]")
			os.Exit(1)
		}
		execBinary(config, binDir, os.Args[2], os.Args[3:])
	default:
		fmt.Println("Unknown command. Use 'bd install' or 'bd exec <name>'")
		os.Exit(1)
	}
}

func loadConfig() (*Config, error) {
	file, err := os.ReadFile("bd.json")
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

	for _, binary := range config.Binaries {
		pkgParts := strings.Split(binary.Package, "@")
		if len(pkgParts) > 1 {
			binary.Package = pkgParts[0]
			if binary.Version == "" {
				binary.Version = pkgParts[1]
			}
			if binary.Version != pkgParts[1] {
				return nil, fmt.Errorf("version mismatch for binary %s", binary.Package)
			}
		}

		if binary.Version == "" {
			binary.Version = "latest"
		}

		if binary.Name == "" {
			pkgSlashParts := strings.Split(binary.Package, "/")
			binary.Name = pkgSlashParts[len(pkgSlashParts)-1]
		}
	}

	return &config, nil
}

func installBinary(bin Binary, binDir string) error {
	outputName := fmt.Sprintf("%s-%s", bin.Name, bin.Version)
	finalPath := filepath.Join(binDir, outputName)

	if _, err := os.Stat(finalPath); err == nil && bin.Version != "latest" {
		fmt.Printf("Already installed: %s\n", finalPath)
		return nil
	}

	fmt.Printf("Installing: %s@%s\n", bin.Package, bin.Version)

	tempDir, err := os.MkdirTemp("", "bd-build")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempBinDir := filepath.Join(tempDir, "bin")
	if err := os.Mkdir(tempBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	cmd := exec.Command("go", "install", fmt.Sprintf("%s@%s", bin.Package, bin.Version))
	cmd.Env = append(os.Environ(), "GOBIN="+tempBinDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install %s: %w", bin.Package, err)
	}

	files, err := os.ReadDir(tempBinDir)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("failed to find built binary: %w", err)
	}
	binaryPath := filepath.Join(tempBinDir, files[0].Name())

	if err := os.Rename(binaryPath, finalPath); err != nil {
		return fmt.Errorf("failed to move binary: %w", err)
	}

	fmt.Printf("Installed: %s\n", finalPath)

	return nil
}

func installBinaries(config *Config, binDir string) error {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create binDir: %w", err)
	}

	for _, bin := range config.Binaries {
		if err := installBinary(*bin, binDir); err != nil {
			return fmt.Errorf("failed to install binary: %w", err)
		}
	}

	return nil
}

func execBinary(config *Config, binDir, name string, args []string) {
	var binPath string
	for _, bin := range config.Binaries {
		if bin.Name == name {
			binPath = filepath.Join(binDir, fmt.Sprintf("%s-%s", bin.Name, bin.Version))
			break
		}
	}

	if binPath == "" {
		_, _ = fmt.Fprintf(os.Stderr, "Binary '%s' not found in bd.yaml", name)
		os.Exit(1)
	}

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		_, _ = fmt.Fprintf(os.Stderr, "Binary '%s' is not installed. Run 'bd install' first.\n", name)
		os.Exit(1)
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to execute %s: %v\n", name, err)
		os.Exit(1)
	}
}
