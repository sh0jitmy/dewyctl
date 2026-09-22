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
	"runtime"
	"strings"

	"github.com/sh0jitmy/dewyctl/internal/builder"
	"github.com/sh0jitmy/dewyctl/internal/s3"
	"github.com/sh0jitmy/dewyctl/internal/sops"
)

// RunPush executes the push subcommand to build and upload binary to object storage
func RunPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	version := fs.String("version", "", "Semantic version tag (required, e.g. v1.0.0)")
	appName := fs.String("app", "", "Application name (default: auto-detected from .goreleaser.yaml or dir)")
	appDir := fs.String("dir", "", "Path to Go application directory (default: auto-detect . or app)")
	bucket := fs.String("bucket", "", "S3 bucket name (or via S3_BUCKET env)")
	endpoint := fs.String("endpoint", "", "S3 endpoint URL (or via S3_ENDPOINT env, default: https://s3.tky01.sakurastorage.jp)")
	region := fs.String("region", "", "S3 region (or via S3_REGION env, default: jp-east-1)")
	prefix := fs.String("prefix", "", "S3 prefix/directory (default: sample-app for app, dewyctl for dewyctl, or via S3_PREFIX env)")
	targetOS := fs.String("os", "", "Target OS (e.g. linux, darwin)")
	targetArch := fs.String("arch", "", "Target architecture (e.g. amd64, arm64)")
	allPlatforms := fs.Bool("all-platforms", false, "Build and upload for all standard platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *version == "" {
		return fmt.Errorf("--version is required (e.g. --version v1.0.0)")
	}
	// Ensure version has 'v' prefix if missing, for standard SemVer
	ver := *version
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}

	// Auto-detect app directory if not specified
	if *appDir == "" {
		if _, err := os.Stat("app/go.mod"); err == nil {
			*appDir = "app"
		} else {
			*appDir = "."
		}
	}

	// Auto-detect app name
	if *appName == "" {
		*appName = builder.DetectProjectName()
	}

	// Load credentials from SOPS (secrets.enc.yml) or environment
	creds, _ := sops.LoadCredentials()

	// Default endpoint to Tokyo region if not provided
	finalEndpoint := *endpoint
	if finalEndpoint == "" {
		finalEndpoint = os.Getenv("S3_ENDPOINT")
		if finalEndpoint == "" {
			finalEndpoint = creds["s3_endpoint"]
			if finalEndpoint == "" {
				finalEndpoint = "https://s3.tky01.sakurastorage.jp"
			}
		}
	}

	finalRegion := *region
	if finalRegion == "" {
		finalRegion = os.Getenv("S3_REGION")
		if finalRegion == "" {
			finalRegion = creds["s3_region"]
			if finalRegion == "" {
				finalRegion = "jp-east-1"
			}
		}
	}

	finalPrefix := *prefix
	if finalPrefix == "" {
		finalPrefix = os.Getenv("S3_PREFIX")
		if finalPrefix == "" {
			finalPrefix = creds["s3_prefix"]
			if finalPrefix == "" {
				if *appName == "dewyctl" {
					finalPrefix = "dewyctl"
				} else {
					finalPrefix = "sample-app"
				}
			}
		}
	}

	finalBucket := *bucket
	if finalBucket == "" {
		finalBucket = os.Getenv("S3_BUCKET")
		if finalBucket == "" {
			finalBucket = creds["s3_bucket"]
		}
	}

	if finalBucket == "" || strings.Contains(finalBucket, "placeholder") {
		if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
			return fmt.Errorf("S3 bucket is not configured (current: '%s').\nIn GitHub Actions, please configure the 'S3_BUCKET' repository secret (or SOPS_AGE_KEY with encrypted secrets)", finalBucket)
		}
		return fmt.Errorf("S3 bucket is not configured (current: '%s').\nPlease run 'dewyctl config' (or './run.sh config') to set up your S3 bucket and credentials", finalBucket)
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	if accessKey == "" {
		accessKey = creds["aws_access_key_id"]
	}
	if accessKey == "" || strings.Contains(accessKey, "placeholder") {
		if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
			return fmt.Errorf("AWS_ACCESS_KEY_ID is not configured (or is a placeholder).\nIn GitHub Actions, please configure the 'AWS_ACCESS_KEY_ID' repository secret (or SOPS_AGE_KEY with encrypted secrets)")
		}
		return fmt.Errorf("AWS_ACCESS_KEY_ID is not configured (or is a placeholder).\nPlease run 'dewyctl config' (or './run.sh config') to set up your S3 credentials")
	}

	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if secretKey == "" {
		secretKey = creds["aws_secret_access_key"]
	}
	if secretKey == "" || strings.Contains(secretKey, "placeholder") {
		if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
			return fmt.Errorf("AWS_SECRET_ACCESS_KEY is not configured (or is a placeholder).\nIn GitHub Actions, please configure the 'AWS_SECRET_ACCESS_KEY' repository secret (or SOPS_AGE_KEY with encrypted secrets)")
		}
		return fmt.Errorf("AWS_SECRET_ACCESS_KEY is not configured (or is a placeholder).\nPlease run 'dewyctl config' (or './run.sh config') to set up your S3 credentials")
	}

	// Ensure AWS SDK picks up the resolved credentials
	_ = os.Setenv("AWS_ACCESS_KEY_ID", accessKey)
	_ = os.Setenv("AWS_SECRET_ACCESS_KEY", secretKey)
	_ = os.Setenv("AWS_DEFAULT_REGION", finalRegion)

	// Determine target platforms to build
	var platforms []builder.Platform
	if *allPlatforms {
		platforms = builder.StandardPlatforms
	} else if *targetOS != "" || *targetArch != "" {
		tos := *targetOS
		if tos == "" {
			tos = "linux"
		}
		tarch := *targetArch
		if tarch == "" {
			tarch = "amd64"
		}
		platforms = []builder.Platform{{OS: tos, Arch: tarch}}
	} else {
		// Default: build linux/amd64 AND current host platform (for local testing on macOS etc.)
		platforms = []builder.Platform{{OS: "linux", Arch: "amd64"}}
		if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
			platforms = append(platforms, builder.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH})
		}
	}

	fmt.Printf("=== dewyctl push: Release %s (%s) ===\n", *appName, ver)
	fmt.Printf(" Target platforms: %d platform(s)\n", len(platforms))

	ctx := context.Background()
	var publishedKeys []string

	for _, p := range platforms {
		// 1. Build and Archive
		data, archiveName, err := builder.BuildAndArchive(builder.BuildOptions{
			AppName:    *appName,
			AppDir:     *appDir,
			Version:    ver,
			TargetOS:   p.OS,
			TargetArch: p.Arch,
		})
		if err != nil {
			return fmt.Errorf("build failed for %s/%s: %w", p.OS, p.Arch, err)
		}

		// 2. Dewy S3 key convention: <prefix>/<semver>/<artifact>
		s3Key := fmt.Sprintf("%s/%s/%s", finalPrefix, ver, archiveName)

		// 3. Upload to Object Storage
		err = s3.UploadArtifact(ctx, s3.UploadOptions{
			Endpoint: finalEndpoint,
			Region:   finalRegion,
			Bucket:   finalBucket,
			Key:      s3Key,
			Data:     data,
		})
		if err != nil {
			return fmt.Errorf("upload to object storage failed for %s: %w", s3Key, err)
		}
		publishedKeys = append(publishedKeys, s3Key)
	}

	fmt.Println("\n==================================================")
	fmt.Printf("Release %s published successfully to Object Storage!\n", ver)
	for _, key := range publishedKeys {
		fmt.Printf(" [+] S3 Key: %s\n", key)
	}
	fmt.Println("Dewy on your servers will detect this new release and perform zero-downtime deployment.")
	fmt.Println("==================================================")

	return nil
}
