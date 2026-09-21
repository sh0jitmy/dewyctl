package builder

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Platform represents target OS and architecture
type Platform struct {
	OS   string
	Arch string
}

// StandardPlatforms represents the target platforms defined in .goreleaser.yaml
var StandardPlatforms = []Platform{
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "darwin", Arch: "amd64"},
	{OS: "darwin", Arch: "arm64"},
}

// DetectProjectName returns project name from .goreleaser.yaml or directory name
func DetectProjectName() string {
	if content, err := os.ReadFile(".goreleaser.yaml"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "project_name:") {
				name := strings.TrimSpace(strings.TrimPrefix(line, "project_name:"))
				if name != "" {
					return name
				}
			}
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return "dewy-app"
	}
	base := filepath.Base(dir)
	if base == "." || base == "/" || base == "" {
		return "dewy-app"
	}
	return base
}

// BuildOptions contains parameters for cross-compilation
type BuildOptions struct {
	AppName    string
	AppDir     string
	Version    string
	TargetOS   string
	TargetArch string
}

// BuildAndArchive compiles the Go application and returns a tar.gz archive
func BuildAndArchive(opts BuildOptions) ([]byte, string, error) {
	if opts.TargetOS == "" {
		opts.TargetOS = "linux"
	}
	if opts.TargetArch == "" {
		opts.TargetArch = "amd64"
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}

	tempDir, err := os.MkdirTemp("", "dewyctl-build-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	binaryPath := filepath.Join(tempDir, opts.AppName)

	fmt.Printf("--> Building %s for %s/%s (Version: %s)...\n", opts.AppName, opts.TargetOS, opts.TargetArch, opts.Version)

	ldflags := fmt.Sprintf("-s -w -X main.Version=%s", opts.Version)
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binaryPath, ".")
	if opts.AppDir != "" {
		cmd.Dir = opts.AppDir
	}
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		fmt.Sprintf("GOOS=%s", opts.TargetOS),
		fmt.Sprintf("GOARCH=%s", opts.TargetArch),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("go build failed: %w", err)
	}

	// Create tar.gz archive
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read compiled binary: %w", err)
	}

	archiveName := fmt.Sprintf("%s_%s_%s.tar.gz", opts.AppName, opts.TargetOS, opts.TargetArch)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	header := &tar.Header{
		Name: opts.AppName,
		Mode: 0755,
		Size: int64(len(binaryData)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return nil, "", fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write(binaryData); err != nil {
		return nil, "", fmt.Errorf("failed to write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gw.Close(); err != nil {
		return nil, "", err
	}

	fmt.Printf("--> Created archive: %s (%d bytes)\n", archiveName, buf.Len())
	return buf.Bytes(), archiveName, nil
}
