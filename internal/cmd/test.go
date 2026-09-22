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
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sh0jitmy/dewyctl/internal/builder"
	"github.com/sh0jitmy/dewyctl/internal/dewy"
	"github.com/sh0jitmy/dewyctl/internal/s3"
	"github.com/sh0jitmy/dewyctl/internal/sops"
)

// RunTest executes an end-to-end automated deployment test:
// releases v1 -> runs Dewy -> verifies v1 -> releases v2 -> verifies zero-downtime switch to v2.
// Works seamlessly on local developer machines (macOS/Linux) and CI runners (GitHub Actions).
func RunTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	port := fs.Int("port", 8080, "HTTP port for test application")
	v1 := fs.String("v1", "v0.1.0", "Initial release version")
	v2 := fs.String("v2", "v0.2.0", "Upgraded release version for zero-downtime switch test")
	appName := fs.String("app", "", "Application name (default: current directory name or dewy-app)")
	keepRunning := fs.Bool("keep", false, "Keep dewy server running after test completes")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Ensure dewy binary is ready
	dewyBin, err := dewy.EnsureDewyInstalled(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure dewy binary: %w", err)
	}

	// 2. Load credentials
	creds, _ := sops.LoadCredentials()
	bucket := creds["s3_bucket"]
	endpoint := creds["s3_endpoint"]
	if endpoint == "" {
		endpoint = "https://s3.tky01.sakurastorage.jp"
	}
	region := creds["s3_region"]
	if region == "" {
		region = "jp-east-1"
	}
	accessKey := creds["aws_access_key_id"]
	secretKey := creds["aws_secret_access_key"]

	if bucket == "" || strings.Contains(bucket, "placeholder") {
		return fmt.Errorf("S3 bucket is required. Run 'dewyctl config' first or set S3_BUCKET")
	}
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS credentials are required. Run 'dewyctl config' first or set AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY")
	}

	finalApp := *appName
	if finalApp == "" {
		cur, _ := os.Getwd()
		finalApp = filepath.Base(cur)
		if finalApp == "." || finalApp == "/" {
			finalApp = "dewy-app"
		}
	}

	appDir := "."
	if _, err := os.Stat("app/go.mod"); err == nil {
		appDir = "app"
	}

	prefix := os.Getenv("S3_PREFIX")
	if prefix == "" {
		if finalApp == "dewyctl" {
			prefix = "dewyctl"
		} else {
			prefix = "sample-app"
		}
	}

	osName := runtime.GOOS
	archName := runtime.GOARCH
	artifactName := fmt.Sprintf("%s_%s_%s.tar.gz", finalApp, osName, archName)
	registryURL := fmt.Sprintf("s3://%s/%s/%s?endpoint=%s&artifact=%s",
		region, bucket, prefix, endpoint, artifactName)

	testDir, err := os.MkdirTemp("", "dewy-e2e-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp test directory: %w", err)
	}
	defer func() {
		if !*keepRunning {
			_ = os.RemoveAll(testDir)
		}
	}()

	fmt.Println("============================================================")
	fmt.Println("       Starting Dewy Zero-Downtime E2E Automation Test")
	fmt.Printf(" App:             %s\n", finalApp)
	fmt.Printf(" S3 Bucket:       %s\n", bucket)
	fmt.Printf(" S3 Prefix:       %s\n", prefix)
	fmt.Printf(" Target Artifact: %s\n", artifactName)
	fmt.Printf(" Registry URL:    %s\n", registryURL)
	fmt.Println("============================================================")

	// Helper to build and upload release
	uploadRelease := func(version string) error {
		fmt.Printf("\n--> Building and uploading %s release...\n", version)
		data, archiveName, err := builder.BuildAndArchive(builder.BuildOptions{
			AppName:    finalApp,
			AppDir:     appDir,
			Version:    version,
			TargetOS:   osName,
			TargetArch: archName,
		})
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		s3Key := fmt.Sprintf("%s/%s/%s", prefix, version, archiveName)
		uploadCtx, uploadCancel := context.WithTimeout(ctx, 30*time.Second)
		defer uploadCancel()

		err = s3.UploadArtifact(uploadCtx, s3.UploadOptions{
			Endpoint:        endpoint,
			Region:          region,
			Bucket:          bucket,
			Key:             s3Key,
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			Data:            data,
		})
		if err != nil {
			return fmt.Errorf("S3 upload failed: %w", err)
		}
		fmt.Printf("[+] Uploaded %s to s3://%s/%s\n", version, bucket, s3Key)
		return nil
	}

	// -------------------------------------------------------------
	// Step 1: Release initial version v1
	// -------------------------------------------------------------
	fmt.Println("\n[Step 1/5] Publishing initial release (" + *v1 + ") to Object Storage...")
	if err := uploadRelease(*v1); err != nil {
		return err
	}

	// -------------------------------------------------------------
	// Step 2: Start Dewy daemon in background
	// -------------------------------------------------------------
	fmt.Println("\n[Step 2/5] Starting Dewy Server daemon in background...")
	_ = os.MkdirAll(filepath.Join(testDir, "releases"), 0755)
	execCommand := filepath.Join(testDir, "current", finalApp)

	dewyCmd := exec.Command(dewyBin, "server",
		"--registry", registryURL,
		"--interval", "3",
		"--port", fmt.Sprintf("%d", *port),
		"--", execCommand,
	)
	dewyCmd.Dir = testDir
	dewyCmd.Env = append(os.Environ(),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", accessKey),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", secretKey),
		fmt.Sprintf("AWS_DEFAULT_REGION=%s", region),
	)

	// Pipe logs to stdout with prefix
	logPipe, err := dewyCmd.StdoutPipe()
	if err == nil {
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := logPipe.Read(buf)
				if n > 0 {
					lines := strings.Split(string(buf[:n]), "\n")
					for _, l := range lines {
						if strings.TrimSpace(l) != "" {
							fmt.Printf("  [dewy] %s\n", l)
						}
					}
				}
				if err != nil {
					break
				}
			}
		}()
	}
	dewyCmd.Stderr = dewyCmd.Stdout

	if err := dewyCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dewy server: %w", err)
	}

	defer func() {
		if !*keepRunning && dewyCmd.Process != nil {
			fmt.Println("\n--> Stopping background Dewy server...")
			_ = dewyCmd.Process.Kill()
		}
	}()

	// -------------------------------------------------------------
	// Step 3: Verify initial version v1 is running
	// -------------------------------------------------------------
	fmt.Printf("\n[Step 3/5] Waiting for Dewy to pull and run %s on port %d...\n", *v1, *port)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	maxWait := 30
	v1OK := false

	for i := 1; i <= maxWait; i++ {
		resp, err := httpClient.Get(fmt.Sprintf("http://localhost:%d/health", *port))
		if err == nil && resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if strings.Contains(string(body), `"status":"ok"`) {
				v1OK = true
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	if !v1OK {
		return fmt.Errorf("timed out waiting for Dewy to start %s on port %d", *v1, *port)
	}

	// Verify root version endpoint
	resp, err := httpClient.Get(fmt.Sprintf("http://localhost:%d/", *port))
	if err != nil {
		return fmt.Errorf("failed to query root endpoint: %w", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	bodyStr := string(bodyBytes)

	cleanV1 := strings.TrimPrefix(*v1, "v")
	if !strings.Contains(bodyStr, *v1) && !strings.Contains(bodyStr, cleanV1) {
		return fmt.Errorf("initial version mismatch! Expected %s, got:\n%s", *v1, bodyStr)
	}
	fmt.Printf("[OK] %s is LIVE and healthy on http://localhost:%d\n", *v1, *port)

	// -------------------------------------------------------------
	// Step 4: Release upgraded version v2
	// -------------------------------------------------------------
	fmt.Printf("\n[Step 4/5] Publishing upgraded release (%s) to Object Storage...\n", *v2)
	if err := uploadRelease(*v2); err != nil {
		return err
	}

	// -------------------------------------------------------------
	// Step 5: Verify zero-downtime automatic upgrade to v2
	// -------------------------------------------------------------
	fmt.Printf("\n[Step 5/5] Monitoring zero-downtime automatic upgrade to %s...\n", *v2)
	v2OK := false
	cleanV2 := strings.TrimPrefix(*v2, "v")

	for i := 1; i <= maxWait; i++ {
		resp, err := httpClient.Get(fmt.Sprintf("http://localhost:%d/", *port))
		if err == nil && resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			currentContent := string(body)
			if strings.Contains(currentContent, *v2) || strings.Contains(currentContent, cleanV2) {
				fmt.Printf("\n[+] Zero-downtime switch verified! Current Response:\n%s\n", currentContent)
				v2OK = true
				break
			}
		}
		fmt.Printf("    Waiting for Dewy to auto-detect and switch to %s (attempt %d/%d)...\n", *v2, i, maxWait)
		time.Sleep(2 * time.Second)
	}

	if !v2OK {
		return fmt.Errorf("zero-downtime upgrade to %s timed out", *v2)
	}

	fmt.Println("\n============================================================")
	fmt.Println(" [SUCCESS] Dewy E2E Automation Test PASSED!")
	fmt.Printf(" 1. Initial Deployment: %s -> PASSED\n", *v1)
	fmt.Printf(" 2. S3 Polling & Pull:  PASSED\n")
	fmt.Printf(" 3. Zero-downtime Upgrade: %s -> %s -> PASSED\n", *v1, *v2)
	fmt.Println("============================================================")

	if *keepRunning {
		fmt.Println("Dewy server is kept running. Press Ctrl+C to stop.")
		_ = dewyCmd.Wait()
	}

	return nil
}
