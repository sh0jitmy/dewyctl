// Copyright (c) 2026 sh0jitmy <shjtmy@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dewy

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	DefaultDewyVersion = "2.14.1"
)

// EnsureDewyInstalled checks if dewy is available on PATH or in local cache,
// and downloads the official binary from GitHub Releases if missing.
func EnsureDewyInstalled(ctx context.Context) (string, error) {
	// 1. Check system PATH
	if p, err := exec.LookPath("dewy"); err == nil {
		return p, nil
	}

	// 2. Check local project cache (.dewy/bin/dewy)
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".dewy", "bin")
	localDewy := filepath.Join(cacheDir, "dewy")
	if runtime.GOOS == "windows" {
		localDewy += ".exe"
	}

	if _, err := os.Stat(localDewy); err == nil {
		return localDewy, nil
	}

	// 3. Download from GitHub Releases
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Map architectures if needed
	switch archName {
	case "amd64", "arm64":
		// Matches dewy release naming
	default:
		return "", fmt.Errorf("unsupported architecture for dewy: %s", archName)
	}

	archiveName := fmt.Sprintf("dewy_%s_%s.tar.gz", osName, archName)
	downloadURL := fmt.Sprintf("https://github.com/linyows/dewy/releases/download/v%s/%s", DefaultDewyVersion, archiveName)

	fmt.Printf("--> dewy command not found. Automatically downloading dewy v%s (%s/%s)...\n", DefaultDewyVersion, osName, archName)
	fmt.Printf("    URL: %s\n", downloadURL)

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download dewy from %s: %w", downloadURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download dewy (HTTP %d): %s", resp.StatusCode, downloadURL)
	}

	// Extract binary from tar.gz
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read gzip archive: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	found := false

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}

		baseName := filepath.Base(header.Name)
		if baseName == "dewy" || baseName == "dewy.exe" {
			outFile, err := os.OpenFile(localDewy, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return "", fmt.Errorf("failed to create binary file %s: %w", localDewy, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				_ = outFile.Close()
				return "", fmt.Errorf("failed to extract binary: %w", err)
			}
			_ = outFile.Close()
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("binary 'dewy' not found in downloaded archive")
	}

	fmt.Printf("[+] dewy v%s installed successfully to %s\n", DefaultDewyVersion, localDewy)
	return localDewy, nil
}
