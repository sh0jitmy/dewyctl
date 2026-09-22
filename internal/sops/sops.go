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

package sops

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureAgeKey ensures an age keypair exists, creating one if necessary
func EnsureAgeKey(keyFilePath string) (publicKey string, privateKey string, err error) {
	if _, err := os.Stat(keyFilePath); err == nil {
		// Key already exists, read it
		content, err := os.ReadFile(keyFilePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read existing key file %s: %w", keyFilePath, err)
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# public key:") {
				publicKey = strings.TrimSpace(strings.TrimPrefix(line, "# public key:"))
			} else if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
				privateKey = line
			}
		}
		if publicKey != "" && privateKey != "" {
			return publicKey, privateKey, nil
		}
	}

	// Generate key using age-keygen
	ageKeygen, err := exec.LookPath("age-keygen")
	if err != nil {
		return "", "", fmt.Errorf("age-keygen not found in PATH. Please install age (e.g. brew install age)")
	}

	cmd := exec.Command(ageKeygen, "-o", keyFilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("failed to run age-keygen: %s (%w)", string(output), err)
	}

	content, err := os.ReadFile(keyFilePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read newly created key file %s: %w", keyFilePath, err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# public key:") {
			publicKey = strings.TrimSpace(strings.TrimPrefix(line, "# public key:"))
		} else if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
			privateKey = line
		}
	}

	if publicKey == "" || privateKey == "" {
		return "", "", fmt.Errorf("failed to parse generated age key from %s", keyFilePath)
	}

	return publicKey, privateKey, nil
}

// GenerateSopsConfig writes a standard .sops.yaml with the given age public key
func GenerateSopsConfig(publicKey, sopsConfigFile string) error {
	tmpl := fmt.Sprintf(`creation_rules:
  - age: >-
      %s
`, publicKey)
	return os.WriteFile(sopsConfigFile, []byte(tmpl), 0644)
}

// EnsureGitignoreEntries appends required entries to .gitignore if not already present
func EnsureGitignoreEntries(entries []string) error {
	gitignorePath := ".gitignore"
	existing := make(map[string]bool)

	if data, err := os.ReadFile(gitignorePath); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			existing[strings.TrimSpace(scanner.Text())] = true
		}
	}

	var toAdd []string
	for _, e := range entries {
		if !existing[e] && !existing["/"+e] {
			toAdd = append(toAdd, e)
		}
	}

	if len(toAdd) > 0 {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		for _, item := range toAdd {
			if _, err := f.WriteString(item + "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// EncryptTemplate encrypts plaintext content using SOPS and writes to encPath
func EncryptTemplate(keyFilePath, encPath, plainContent string) error {
	sopsBin, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops not found in PATH. Please install sops (e.g. brew install sops)")
	}

	tempFile := filepath.Join(os.TempDir(), "sops-plain-secrets.yml")
	if err := os.WriteFile(tempFile, []byte(plainContent), 0600); err != nil {
		return fmt.Errorf("failed to write temporary secret file: %w", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

	absKey, _ := filepath.Abs(keyFilePath)
	cmd := exec.Command(sopsBin, "--encrypt", tempFile)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOPS_AGE_KEY_FILE=%s", absKey))

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("sops encryption failed: %s", string(exitErr.Stderr))
		}
		return fmt.Errorf("sops encryption failed: %w", err)
	}

	return os.WriteFile(encPath, output, 0644)
}

// Decrypt runs sops --decrypt and outputs to stdout or file
func Decrypt(keyFilePath, encPath string) (string, error) {
	sopsBin, err := exec.LookPath("sops")
	if err != nil {
		return "", fmt.Errorf("sops not found in PATH")
	}
	absKey, _ := filepath.Abs(keyFilePath)
	cmd := exec.Command(sopsBin, "--decrypt", encPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOPS_AGE_KEY_FILE=%s", absKey))

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("sops decryption failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("sops decryption failed: %w", err)
	}
	return string(output), nil
}

// Edit runs sops in interactive edit mode
func Edit(keyFilePath, encPath string) error {
	sopsBin, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops not found in PATH")
	}
	absKey, _ := filepath.Abs(keyFilePath)
	cmd := exec.Command(sopsBin, encPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOPS_AGE_KEY_FILE=%s", absKey))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// LoadCredentials decrypts secrets.enc.yml and returns key-value map, falling back to env vars
func LoadCredentials() (map[string]string, error) {
	creds := make(map[string]string)

	keyFile := "key.txt"
	encFile := "secrets.enc.yml"

	// If SOPS_AGE_KEY is provided in env, create temp keyfile if key.txt does not exist
	if _, err := os.Stat(keyFile); os.IsNotExist(err) && os.Getenv("SOPS_AGE_KEY") != "" {
		tempKey := filepath.Join(os.TempDir(), fmt.Sprintf("sops-key-%d.txt", os.Getpid()))
		_ = os.WriteFile(tempKey, []byte(os.Getenv("SOPS_AGE_KEY")), 0600)
		defer func() { _ = os.Remove(tempKey) }()
		keyFile = tempKey
	}

	// 1. Try decrypting secrets.enc.yml first
	if _, err := os.Stat(encFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			if decrypted, err := Decrypt(keyFile, encFile); err == nil {
				lines := strings.Split(decrypted, "\n")
				for _, line := range lines {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						k := strings.TrimSpace(parts[0])
						v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
						if v != "" && !strings.Contains(v, "placeholder") {
							creds[k] = v
						}
					}
				}
			}
		}
	}

	// 2. Override with environment variables (env vars have the highest precedence)
	envKeys := []string{
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "S3_BUCKET",
		"S3_ENDPOINT", "S3_REGION", "S3_PREFIX", "GITHUB_TOKEN",
	}
	for _, k := range envKeys {
		if v := os.Getenv(k); v != "" {
			creds[strings.ToLower(k)] = v
		}
	}

	// 3. Set standard defaults if not present
	if creds["s3_endpoint"] == "" {
		creds["s3_endpoint"] = "https://s3.tky01.sakurastorage.jp"
	}
	if creds["s3_region"] == "" {
		creds["s3_region"] = "jp-east-1"
	}
	if creds["s3_prefix"] == "" {
		creds["s3_prefix"] = "sample-app"
	}

	return creds, nil
}
