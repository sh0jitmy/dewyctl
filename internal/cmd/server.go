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

package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sh0jitmy/dewyctl/internal/builder"
	"github.com/sh0jitmy/dewyctl/internal/dewy"
	"github.com/sh0jitmy/dewyctl/internal/sops"
)

// RunServer launches the dewy server daemon on the current host (macOS, Linux, or CI)
func RunServer(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	port := fs.Int("port", 8080, "Application listening port")
	interval := fs.Int("interval", 5, "Polling interval in seconds")
	appName := fs.String("app", "", "Application name (default: directory name or dewy-app)")
	bucket := fs.String("bucket", "", "S3 bucket name (default: from secrets/env)")
	endpoint := fs.String("endpoint", "", "S3 endpoint URL (default: from secrets/env)")
	region := fs.String("region", "", "S3 region (default: from secrets/env)")
	prefix := fs.String("prefix", "app", "S3 prefix (default: app)")
	workDir := fs.String("work-dir", ".dewy", "Working directory for releases and symlinks")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	ctx := context.Background()

	// 1. Ensure dewy binary is available
	dewyBin, err := dewy.EnsureDewyInstalled(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure dewy binary: %w", err)
	}

	// 2. Load credentials
	creds, _ := sops.LoadCredentials()

	finalBucket := *bucket
	if finalBucket == "" {
		finalBucket = creds["s3_bucket"]
	}
	if finalBucket == "" || strings.Contains(finalBucket, "placeholder") {
		return fmt.Errorf("S3 bucket is required. Run 'dewyctl config' first or pass --bucket")
	}

	finalEndpoint := *endpoint
	if finalEndpoint == "" {
		finalEndpoint = creds["s3_endpoint"]
		if finalEndpoint == "" {
			finalEndpoint = "https://s3.tky01.sakurastorage.jp"
		}
	}

	finalRegion := *region
	if finalRegion == "" {
		finalRegion = creds["s3_region"]
		if finalRegion == "" {
			finalRegion = "jp-east-1"
		}
	}

	accessKey := creds["aws_access_key_id"]
	secretKey := creds["aws_secret_access_key"]
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required. Run 'dewyctl config' first")
	}

	finalApp := *appName
	if finalApp == "" {
		finalApp = builder.DetectProjectName()
	}

	// 3. Resolve artifact name matching current OS and architecture
	osName := runtime.GOOS
	archName := runtime.GOARCH
	artifactName := fmt.Sprintf("%s_%s_%s.tar.gz", finalApp, osName, archName)

	// 4. Construct Dewy S3 registry URL
	// Format: s3://<region>/<bucket>/<prefix>?endpoint=<endpoint>&artifact=<artifact>
	registryURL := fmt.Sprintf("s3://%s/%s/%s?endpoint=%s&artifact=%s",
		finalRegion, finalBucket, *prefix, finalEndpoint, artifactName)

	absWorkDir, err := filepath.Abs(*workDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(absWorkDir, "releases"), 0755); err != nil {
		return fmt.Errorf("failed to create releases directory: %w", err)
	}

	execCommand := filepath.Join(absWorkDir, "current", finalApp)

	fmt.Println("============================================================")
	fmt.Println("           Starting Dewy Server Daemon")
	fmt.Println("============================================================")
	fmt.Printf(" Host Platform:   %s/%s\n", osName, archName)
	fmt.Printf(" Target Artifact: %s\n", artifactName)
	fmt.Printf(" Registry URL:    %s\n", registryURL)
	fmt.Printf(" Application Port:%d\n", *port)
	fmt.Printf(" Polling Interval:%ds\n", *interval)
	fmt.Printf(" Working Dir:     %s\n", absWorkDir)
	fmt.Printf(" Binary Target:   %s\n", execCommand)
	fmt.Println("============================================================")
	fmt.Println("Dewy is now polling Object Storage for new releases. Press Ctrl+C to stop.")

	cmd := exec.CommandContext(ctx, dewyBin, "server",
		"--registry", registryURL,
		"--interval", fmt.Sprintf("%d", *interval),
		"--port", fmt.Sprintf("%d", *port),
		"--", execCommand,
	)
	cmd.Dir = absWorkDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", accessKey),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", secretKey),
		fmt.Sprintf("AWS_DEFAULT_REGION=%s", finalRegion),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
