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
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/sh0jitmy/dewyctl/internal/sops"
)

// RunDoctor performs diagnostics on environment, configurations, and S3 connectivity
func RunDoctor(args []string) error {
	fmt.Println("=== dewyctl doctor: Environment & Configuration Diagnostic ===")

	allPass := true
	check := func(name string, ok bool, details string) {
		if ok {
			fmt.Printf("[OK]   %-25s %s\n", name, details)
		} else {
			fmt.Printf("[FAIL] %-25s %s\n", name, details)
			allPass = false
		}
	}

	// 1. Check Go compiler
	_, err := exec.LookPath("go")
	check("Go compiler", err == nil, "go is installed and in PATH")

	// 2. Check credentials (from environment or SOPS secrets.enc.yml)
	creds, _ := sops.LoadCredentials()

	ak := creds["aws_access_key_id"]
	akSource := "env"
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" && ak != "" {
		akSource = "SOPS secrets.enc.yml"
	}
	check("AWS_ACCESS_KEY_ID", ak != "", fmt.Sprintf("%s (source: %s)", mask(ak), akSource))

	sk := creds["aws_secret_access_key"]
	skSource := "env"
	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" && sk != "" {
		skSource = "SOPS secrets.enc.yml"
	}
	check("AWS_SECRET_ACCESS_KEY", sk != "", fmt.Sprintf("%s (source: %s)", mask(sk), skSource))

	endpoint := creds["s3_endpoint"]
	if endpoint == "" {
		endpoint = "https://s3.tky01.sakurastorage.jp"
	}
	check("S3_ENDPOINT", endpoint != "", endpoint)

	bucket := creds["s3_bucket"]
	check("S3_BUCKET", bucket != "", bucket)

	region := creds["s3_region"]
	if region == "" {
		region = "jp-east-1"
	}
	check("S3_REGION", region != "", region)

	// 3. Test HTTP Connectivity to S3 Endpoint
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		check("S3 Connectivity", false, fmt.Sprintf("Failed to reach %s: %v", endpoint, err))
	} else {
		_ = resp.Body.Close()
		check("S3 Connectivity", true, fmt.Sprintf("Successfully connected to %s (Status: %d)", endpoint, resp.StatusCode))
	}

	// 4. Check project config files
	_, goreleaserErr := os.Stat(".goreleaser.yaml")
	check(".goreleaser.yaml", goreleaserErr == nil, "Found GoReleaser config")

	_, tagprErr := os.Stat(".github/workflows/tagpr.yml")
	check("tagpr workflow", tagprErr == nil, "Found .github/workflows/tagpr.yml")

	_, dewyBinErr := exec.LookPath("dewy")
	if dewyBinErr == nil {
		check("dewy binary", true, "dewy is installed on host")
	} else {
		check("dewy binary", true, "not found on local host (normal if server-side only)")
	}

	fmt.Println("==================================================")
	if allPass {
		fmt.Println("Diagnostic finished: All critical checks passed!")
	} else {
		fmt.Println("Diagnostic finished: Some items need attention. Check logs above.")
	}

	return nil
}

func mask(s string) string {
	if len(s) == 0 {
		return "missing"
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + stringsRepeat("*", len(s)-4) + s[len(s)-2:]
}

func stringsRepeat(s string, count int) string {
	var res string
	for i := 0; i < count; i++ {
		res += s
	}
	return res
}
