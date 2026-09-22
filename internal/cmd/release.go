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
	"strings"

	"github.com/sh0jitmy/dewyctl/internal/sops"
)

// RunRelease triggers GoReleaser with credentials automatically injected from SOPS,
// falling back to built-in push if goreleaser is not installed.
func RunRelease(args []string) error {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	version := fs.String("version", "", "Semantic release version tag (e.g. v0.1.0)")
	prefix := fs.String("prefix", "", "S3 prefix/directory (default: dewyctl or S3_PREFIX env)")
	skipPublish := fs.Bool("skip-publish", false, "Skip S3 upload and only build archives")
	useFallback := fs.Bool("builtin", false, "Use dewyctl built-in cross-compiler instead of GoReleaser")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if *version == "" {
		return fmt.Errorf("--version is required (e.g. --version v0.1.0)")
	}
	ver := *version
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}

	// 1. Load credentials from SOPS
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
	finalPrefix := *prefix
	if finalPrefix == "" {
		finalPrefix = os.Getenv("S3_PREFIX")
		if finalPrefix == "" {
			finalPrefix = creds["s3_prefix"]
			if finalPrefix == "" {
				finalPrefix = "dewyctl"
			}
		}
	}
	accessKey := creds["aws_access_key_id"]
	secretKey := creds["aws_secret_access_key"]

	// Set process environment for child tools
	if accessKey != "" {
		_ = os.Setenv("AWS_ACCESS_KEY_ID", accessKey)
	}
	if secretKey != "" {
		_ = os.Setenv("AWS_SECRET_ACCESS_KEY", secretKey)
	}
	if bucket != "" {
		_ = os.Setenv("S3_BUCKET", bucket)
	}
	if endpoint != "" {
		_ = os.Setenv("S3_ENDPOINT", endpoint)
	}
	if region != "" {
		_ = os.Setenv("S3_REGION", region)
	}
	_ = os.Setenv("S3_PREFIX", finalPrefix)

	// 2. Check if GoReleaser is available
	goreleaserBin, goreleaserErr := exec.LookPath("goreleaser")

	if *useFallback || goreleaserErr != nil {
		if goreleaserErr != nil && !*useFallback {
			fmt.Println("--> 'goreleaser' command not found. Using dewyctl built-in cross-compiler & S3 uploader...")
		}
		// Built-in push fallback (build for all standard platforms just like GoReleaser does)
		return RunPush([]string{
			"--version", ver,
			"--bucket", bucket,
			"--endpoint", endpoint,
			"--region", region,
			"--prefix", finalPrefix,
			"--all-platforms",
		})
	}

	// 3. Run GoReleaser
	fmt.Println("============================================================")
	fmt.Printf(" Running GoReleaser Release (%s)\n", ver)
	fmt.Printf(" S3 Bucket:   %s\n", bucket)
	fmt.Printf(" S3 Prefix:   %s\n", finalPrefix)
	fmt.Printf(" S3 Endpoint: %s\n", endpoint)
	fmt.Println("============================================================")

	cmdArgs := []string{"release", "--clean"}
	if *skipPublish {
		cmdArgs = append(cmdArgs, "--skip=publish")
	} else {
		// Skip GitHub announce and validation when running locally or in standalone CI
		cmdArgs = append(cmdArgs, "--skip=announce")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, goreleaserBin, cmdArgs...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("S3_BUCKET=%s", bucket),
		fmt.Sprintf("S3_PREFIX=%s", finalPrefix),
		fmt.Sprintf("S3_REGION=%s", region),
		fmt.Sprintf("S3_ENDPOINT=%s", endpoint),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", accessKey),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", secretKey),
		fmt.Sprintf("AWS_DEFAULT_REGION=%s", region),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goreleaser failed: %w", err)
	}

	fmt.Println("\n[+] GoReleaser release completed successfully!")
	return nil
}
