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
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sh0jitmy/dewyctl/internal/template"
)

// RunInit executes the init subcommand
func RunInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	appName := fs.String("app", "", "Application name (default: current directory name)")
	port := fs.String("port", "8080", "Application listening port")
	bucket := fs.String("bucket", "my-sakura-bucket", "S3 bucket name")
	endpoint := fs.String("endpoint", "https://s3.tky01.sakurastorage.jp", "S3 endpoint URL")
	region := fs.String("region", "jp-east-1", "S3 region")
	binaryDir := fs.String("dest", "", "Binary destination directory on target server (default: /opt/<app>)")
	nonInteractive := fs.Bool("yes", false, "Non-interactive mode (use defaults)")
	withSops := fs.Bool("sops", false, "Initialize SOPS and age encryption")
	withAnsible := fs.Bool("ansible", false, "Generate Ansible playbooks and roles")

	if err := fs.Parse(args); err != nil {
		return err
	}

	currentDir, _ := os.Getwd()
	defaultAppName := filepath.Base(currentDir)
	if *appName == "" {
		*appName = defaultAppName
	}

	reader := bufio.NewReader(os.Stdin)
	prompt := func(msg, def string) string {
		if *nonInteractive {
			return def
		}
		fmt.Printf("%s [%s]: ", msg, def)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return def
		}
		return input
	}

	promptBool := func(msg string, def bool) bool {
		if *nonInteractive {
			return def
		}
		defStr := "y/N"
		if def {
			defStr = "Y/n"
		}
		fmt.Printf("%s [%s]: ", msg, defStr)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "" {
			return def
		}
		return input == "y" || input == "yes"
	}

	fmt.Println("=== dewyctl: Initialize Dewy Deployment Configuration ===")
	finalApp := prompt("Application Name", *appName)
	finalPort := prompt("HTTP Port", *port)
	finalBucket := prompt("S3 Bucket Name", *bucket)
	finalEndpoint := prompt("S3 Endpoint URL", *endpoint)
	finalRegion := prompt("S3 Region", *region)
	defaultBinaryDir := *binaryDir
	if defaultBinaryDir == "" {
		defaultBinaryDir = fmt.Sprintf("/opt/%s", finalApp)
	}
	finalBinaryDir := prompt("Destination Directory on Server", defaultBinaryDir)

	doSops := *withSops || promptBool("Setup SOPS + age secret encryption?", true)
	doAnsible := *withAnsible || promptBool("Generate Ansible playbooks & roles?", true)

	cfg := template.Config{
		AppName:    finalApp,
		Port:       finalPort,
		S3Bucket:   finalBucket,
		S3Endpoint: finalEndpoint,
		S3Region:   finalRegion,
		BinaryDir:  finalBinaryDir,
	}

	// 1. Generate systemd service file
	systemdDir := "systemd"
	_ = os.MkdirAll(systemdDir, 0755)
	serviceFile := filepath.Join(systemdDir, fmt.Sprintf("%s.service", finalApp))
	if err := os.WriteFile(serviceFile, []byte(template.SystemdServiceTemplate(cfg)), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", serviceFile, err)
	}
	fmt.Printf("[+] Created %s\n", serviceFile)

	// 2. Generate .goreleaser.yaml
	goreleaserFile := ".goreleaser.yaml"
	if _, err := os.Stat(goreleaserFile); os.IsNotExist(err) {
		if err := os.WriteFile(goreleaserFile, []byte(template.GoReleaserTemplate(cfg)), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", goreleaserFile, err)
		}
		fmt.Printf("[+] Created %s\n", goreleaserFile)
	} else {
		fmt.Printf("[!] %s already exists, skipping.\n", goreleaserFile)
	}

	// 3. Generate dewy.env.example
	envFile := "dewy.env.example"
	if err := os.WriteFile(envFile, []byte(template.EnvFileTemplate(cfg)), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", envFile, err)
	}
	fmt.Printf("[+] Created %s\n", envFile)

	// 4. Generate GitHub Actions workflow for tagpr + S3 release
	workflowDir := filepath.Join(".github", "workflows")
	_ = os.MkdirAll(workflowDir, 0755)
	workflowFile := filepath.Join(workflowDir, "tagpr-release.yml")
	if _, err := os.Stat(workflowFile); os.IsNotExist(err) {
		if err := os.WriteFile(workflowFile, []byte(template.TagprWorkflowTemplate(cfg)), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", workflowFile, err)
		}
		fmt.Printf("[+] Created %s\n", workflowFile)
	} else {
		fmt.Printf("[!] %s already exists, skipping.\n", workflowFile)
	}

	// 5. Generate Go starter snippet if in a Go project
	if _, err := os.Stat("go.mod"); err == nil {
		starterFile := "dewy_starter.go.example"
		if err := os.WriteFile(starterFile, []byte(template.StarterGoSnippet()), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", starterFile, err)
		}
		fmt.Printf("[+] Created %s (reference for socket inheritance)\n", starterFile)
	}

	// 6. Setup SOPS if requested
	if doSops {
		fmt.Println("\n--- Setting up SOPS + age secret encryption ---")
		if err := runSopsInit([]string{
			"-endpoint", finalEndpoint,
			"-bucket", finalBucket,
			"-region", finalRegion,
		}); err != nil {
			fmt.Printf("[!] Warning: SOPS setup encountered an issue: %v\n", err)
		}
	}

	// 7. Setup Ansible if requested
	if doAnsible {
		fmt.Println("\n--- Generating Ansible Playbooks & Roles ---")
		if err := runAnsibleInit([]string{
			"-app", finalApp,
			"-port", finalPort,
			"-bucket", finalBucket,
			"-endpoint", finalEndpoint,
			"-region", finalRegion,
			"-dest", finalBinaryDir,
		}); err != nil {
			fmt.Printf("[!] Warning: Ansible generation encountered an issue: %v\n", err)
		}
	}

	fmt.Println("\nDewy configuration initialized successfully!")
	fmt.Println("Next steps:")
	fmt.Println("  1. Check dewy.env.example or secrets.enc.yml and set your credentials.")
	fmt.Println("  2. Place systemd service in /etc/systemd/system/ (or apply via Ansible).")
	fmt.Println("  3. Run 'dewyctl push --version v0.1.0' to publish your first binary to object storage!")

	return nil
}
