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
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sh0jitmy/dewyctl/internal/s3"
	"github.com/sh0jitmy/dewyctl/internal/sops"
)

// RunSops executes the sops management subcommand
func RunSops(args []string) error {
	if len(args) == 0 {
		printSopsUsage()
		return nil
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "init":
		return runSopsInit(subArgs)
	case "config", "configure", "set":
		return RunSopsConfig(subArgs)
	case "decrypt":
		return runSopsDecrypt(subArgs)
	case "edit":
		return runSopsEdit(subArgs)
	case "help", "-h", "--help":
		printSopsUsage()
		return nil
	default:
		return fmt.Errorf("unknown sops subcommand: %s (run 'dewyctl sops help' for usage)", sub)
	}
}

func printSopsUsage() {
	fmt.Println("Usage: dewyctl sops <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  config   Interactively configure S3 credentials and bucket, then encrypt with SOPS")
	fmt.Println("  init     Generate age keypair, .sops.yaml, and initial secrets.enc.yml")
	fmt.Println("  decrypt  Decrypt secrets.enc.yml to stdout")
	fmt.Println("  edit     Interactively edit secrets.enc.yml in plaintext")
}

// RunSopsConfig interactively sets S3 credentials and encrypts secrets.enc.yml
func RunSopsConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	optBucket := fs.String("bucket", "", "S3 bucket name")
	optAccessKey := fs.String("access-key", "", "S3 Access Key ID")
	optSecretKey := fs.String("secret-key", "", "S3 Secret Access Key")
	optEndpoint := fs.String("endpoint", "", "S3 endpoint URL")
	optRegion := fs.String("region", "", "S3 region")
	optNonInteractive := fs.Bool("non-interactive", false, "Do not prompt for input")
	optSkipTest := fs.Bool("skip-test", false, "Skip S3 bucket connectivity test")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	keyFile := "key.txt"
	encFile := "secrets.enc.yml"

	// 1. Ensure age key and .sops.yaml
	pubKey, privKey, err := sops.EnsureAgeKey(keyFile)
	if err != nil {
		return fmt.Errorf("failed to setup age key: %w", err)
	}
	if _, err := os.Stat(".sops.yaml"); os.IsNotExist(err) {
		_ = sops.GenerateSopsConfig(pubKey, ".sops.yaml")
	}
	_ = sops.EnsureGitignoreEntries([]string{keyFile, "secrets.yml", "*.dec.yml"})

	// 2. Parse existing values if available
	curBucket := ""
	curAccessKey := ""
	curSecretKey := ""
	curEndpoint := "https://s3.tky01.sakurastorage.jp"
	curRegion := "jp-east-1"
	curGithubToken := ""
	curSakuraToken := ""
	curSakuraSecret := ""
	curSakuraZone := "is1b"
	curDeployIP := "127.0.0.1"

	if _, err := os.Stat(encFile); err == nil {
		if decrypted, err := sops.Decrypt(keyFile, encFile); err == nil {
			lines := strings.Split(decrypted, "\n")
			for _, line := range lines {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.TrimSpace(parts[1])
					// Strip quotes
					v = strings.Trim(v, "\"'")
					switch k {
					case "s3_bucket":
						curBucket = v
					case "aws_access_key_id":
						curAccessKey = v
					case "aws_secret_access_key":
						curSecretKey = v
					case "s3_endpoint":
						if v != "" {
							curEndpoint = v
						}
					case "s3_region":
						if v != "" {
							curRegion = v
						}
					case "github_token":
						curGithubToken = v
					case "sakura_access_token":
						curSakuraToken = v
					case "sakura_access_token_secret":
						curSakuraSecret = v
					case "sakura_zone":
						if v != "" {
							curSakuraZone = v
						}
					case "deploy_target_ip":
						if v != "" {
							curDeployIP = v
						}
					}
				}
			}
		}
	}

	// Filter out placeholder markers
	if strings.Contains(curBucket, "placeholder") {
		curBucket = ""
	}
	if strings.Contains(curAccessKey, "placeholder") {
		curAccessKey = ""
	}
	if strings.Contains(curSecretKey, "placeholder") {
		curSecretKey = ""
	}

	// Fallback to environment variables if still empty
	if curBucket == "" && os.Getenv("S3_BUCKET") != "" {
		curBucket = os.Getenv("S3_BUCKET")
	}
	if curAccessKey == "" && os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		curAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if curSecretKey == "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		curSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if os.Getenv("S3_ENDPOINT") != "" {
		curEndpoint = os.Getenv("S3_ENDPOINT")
	}
	if os.Getenv("S3_REGION") != "" {
		curRegion = os.Getenv("S3_REGION")
	}

	// Override from CLI flags if given
	if *optBucket != "" {
		curBucket = *optBucket
	}
	if *optAccessKey != "" {
		curAccessKey = *optAccessKey
	}
	if *optSecretKey != "" {
		curSecretKey = *optSecretKey
	}
	if *optEndpoint != "" {
		curEndpoint = *optEndpoint
	}
	if *optRegion != "" {
		curRegion = *optRegion
	}

	finalBucket := curBucket
	finalAccessKey := curAccessKey
	finalSecretKey := curSecretKey
	finalEndpoint := curEndpoint
	finalRegion := curRegion

	if !*optNonInteractive {
		reader := bufio.NewReader(os.Stdin)
		prompt := func(msg, def string, isSecret bool) string {
			displayDef := def
			if isSecret && def != "" {
				displayDef = "****" + def[max(0, len(def)-4):]
			}
			if displayDef != "" {
				fmt.Printf("%s [%s]: ", msg, displayDef)
			} else {
				fmt.Printf("%s: ", msg)
			}
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				return def
			}
			return input
		}

		fmt.Println("============================================================")
		fmt.Println("  Configure Object Storage (S3) & Credentials for Dewy")
		fmt.Println("============================================================")
		fmt.Println("Enter your Sakura Cloud / S3 Object Storage credentials.")
		fmt.Println("(Press Enter to keep the current/default value in brackets)")
		fmt.Println("")

		finalBucket = prompt("S3 Bucket Name (e.g. my-app-releases)", curBucket, false)
		finalAccessKey = prompt("S3 Access Key ID", curAccessKey, false)
		finalSecretKey = prompt("S3 Secret Access Key", curSecretKey, true)
		finalEndpoint = prompt("S3 Endpoint URL", curEndpoint, false)
		finalRegion = prompt("S3 Region", curRegion, false)
	}

	if finalBucket == "" || finalAccessKey == "" || finalSecretKey == "" {
		return fmt.Errorf("S3 Bucket Name, Access Key ID, and Secret Access Key cannot be empty")
	}

	// 3. Construct Plaintext Secret YAML
	plainYAML := fmt.Sprintf(`github_token: %s
aws_access_key_id: %s
aws_secret_access_key: %s
s3_bucket: %s
s3_region: %s
s3_endpoint: %s
deploy_target_ip: %s
sakura_access_token: %s
sakura_access_token_secret: %s
sakura_zone: %s
tf_state_bucket: %s
tf_state_key: terraform.tfstate
`, finalGithubToken(curGithubToken),
		finalAccessKey,
		finalSecretKey,
		finalBucket,
		finalRegion,
		finalEndpoint,
		curDeployIP,
		curSakuraToken,
		curSakuraSecret,
		curSakuraZone,
		finalBucket,
	)

	// 4. Encrypt and save to secrets.enc.yml via SOPS
	fmt.Println("\n--> Encrypting credentials with SOPS...")
	if err := sops.EncryptTemplate(keyFile, encFile, plainYAML); err != nil {
		return fmt.Errorf("failed to encrypt secrets with SOPS: %w", err)
	}
	fmt.Printf("[+] Successfully encrypted and saved to %s\n", encFile)

	// 5. Also sync to ansible/group_vars/all/vars.yml if exists
	varsFile := "ansible/group_vars/all/vars.yml"
	if data, err := os.ReadFile(varsFile); err == nil {
		content := string(data)
		content = updateYAMLKey(content, "s3_bucket", fmt.Sprintf(`"%s"`, finalBucket))
		content = updateYAMLKey(content, "s3_endpoint", fmt.Sprintf(`"%s"`, finalEndpoint))
		content = updateYAMLKey(content, "s3_region", fmt.Sprintf(`"%s"`, finalRegion))
		_ = os.WriteFile(varsFile, []byte(content), 0644)
		fmt.Printf("[+] Synced bucket & endpoint to %s\n", varsFile)
	}

	// 6. Test connectivity to S3 Bucket
	if !*optSkipTest {
		fmt.Println("\n--> Testing S3 connection and bucket read/write permissions...")
		ctx := context.Background()
		testErr := s3.CheckBucketAccess(ctx, s3.UploadOptions{
			Endpoint:        finalEndpoint,
			Region:          finalRegion,
			Bucket:          finalBucket,
			AccessKeyID:     finalAccessKey,
			SecretAccessKey: finalSecretKey,
		})
		if testErr != nil {
			fmt.Printf("[!] Warning: Bucket access check returned an error:\n    %v\n", testErr)
			fmt.Println("    Please ensure the bucket exists on Sakura Cloud and permissions are correct.")
		} else {
			fmt.Printf("[OK] S3 Bucket access test PASSED! (Verified read/write on %s)\n", finalBucket)
		}
	}

	fmt.Println("\n============================================================")
	fmt.Println("Credentials configured & encrypted successfully!")
	fmt.Println("CI/CD Secret for GitHub Actions:")
	fmt.Println("  Name:  SOPS_AGE_KEY")
	fmt.Printf("  Value: %s\n", privKey)
	fmt.Println("============================================================")


	return nil
}

func finalGithubToken(token string) string {
	if token == "" || strings.Contains(token, "placeholder") {
		return "placeholder_github_token_here"
	}
	return token
}

func updateYAMLKey(content, key, newValue string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			prefix := line[:strings.Index(line, key+":")]
			lines[i] = fmt.Sprintf("%s%s: %s", prefix, key, newValue)
			break
		}
	}
	return strings.Join(lines, "\n")
}

func runSopsInit(args []string) error {
	fs := flag.NewFlagSet("sops init", flag.ExitOnError)
	interactive := fs.Bool("i", false, "Interactive configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interactive {
		return RunSopsConfig(args)
	}
	return RunSopsConfig(args)
}

func runSopsDecrypt(args []string) error {
	keyFile := "key.txt"
	encFile := "secrets.enc.yml"
	if len(args) > 0 {
		encFile = args[0]
	}
	out, err := sops.Decrypt(keyFile, encFile)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runSopsEdit(args []string) error {
	keyFile := "key.txt"
	encFile := "secrets.enc.yml"
	if len(args) > 0 {
		encFile = args[0]
	}
	return sops.Edit(keyFile, encFile)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
