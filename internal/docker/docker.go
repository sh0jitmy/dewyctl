package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sh0jitmy/dewyctl/internal/sops"
)

const (
	ContainerName = "target-server"
	DockerImage   = "geerlingguy/docker-ubuntu2204-ansible:latest"
)

// EnsureContainer verifies if target-server container is running, or creates/starts it
func EnsureContainer(ctx context.Context) error {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found in PATH. Please install Docker")
	}

	// 1. Check if container exists
	checkCmd := exec.CommandContext(ctx, dockerBin, "inspect", ContainerName)
	if err := checkCmd.Run(); err != nil {
		// Container does not exist, run it
		fmt.Printf("--> Spawning systemd container (%s) from %s...\n", ContainerName, DockerImage)
		runCmd := exec.CommandContext(ctx, dockerBin, "run", "-d",
			"--privileged",
			"--name", ContainerName,
			"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw",
			"--cgroupns=host",
			"-v", "/var/run/docker.sock:/var/run/docker.sock",
			DockerImage,
		)
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		if err := runCmd.Run(); err != nil {
			return fmt.Errorf("failed to start Docker container: %w", err)
		}
		time.Sleep(2 * time.Second)
		return nil
	}

	// 2. Check if container is running
	statusCmd := exec.CommandContext(ctx, dockerBin, "inspect", "-f", "{{.State.Running}}", ContainerName)
	out, err := statusCmd.Output()
	if err == nil && strings.TrimSpace(string(out)) == "true" {
		fmt.Printf("--> Docker container %s is already running.\n", ContainerName)
		return nil
	}

	// Container exists but stopped, start it
	fmt.Printf("--> Starting existing stopped container %s...\n", ContainerName)
	startCmd := exec.CommandContext(ctx, dockerBin, "start", ContainerName)
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("failed to start stopped container: %w", err)
	}
	time.Sleep(1 * time.Second)
	return nil
}

// GetContainerIP retrieves the IP address of target-server container
func GetContainerIP(ctx context.Context) (string, error) {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("docker not found in PATH")
	}

	cmd := exec.CommandContext(ctx, dockerBin, "inspect", "-f",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", ContainerName)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to inspect container IP: %w", err)
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		return "", fmt.Errorf("container IP is empty, please ensure container is running")
	}
	return ip, nil
}

// RunAnsible runs ansible-playbook against the local Docker container
func RunAnsible(ctx context.Context) error {
	ansibleBin, err := exec.LookPath("ansible-playbook")
	if err != nil {
		return fmt.Errorf("ansible-playbook not found in PATH. Please install Ansible (e.g. brew install ansible)")
	}

	keyFile := "key.txt"
	encFile := "secrets.enc.yml"

	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return fmt.Errorf("key file %s not found. Please run 'dewyctl config' first", keyFile)
	}
	if _, err := os.Stat(encFile); os.IsNotExist(err) {
		return fmt.Errorf("secrets file %s not found. Please run 'dewyctl config' first", encFile)
	}

	decrypted, err := sops.Decrypt(keyFile, encFile)
	if err != nil {
		return fmt.Errorf("failed to decrypt secrets: %w", err)
	}

	// Warn if placeholder exists
	if strings.Contains(decrypted, "placeholder") {
		fmt.Println("[!] Warning: secrets.enc.yml contains placeholders.")
		fmt.Println("    Please run 'dewyctl config' to set your real S3 bucket and credentials.")
	}

	// Write decrypted secrets to temporary file for Ansible
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("dewy-ansible-secrets-%d.yml", time.Now().UnixNano()))
	if err := os.WriteFile(tempFile, []byte(decrypted), 0600); err != nil {
		return fmt.Errorf("failed to write temporary secrets file: %w", err)
	}
	defer os.Remove(tempFile)

	inventoryFile := "ansible/inventory.ini"
	playbookFile := "ansible/playbook-binary.yml"

	if _, err := os.Stat(inventoryFile); os.IsNotExist(err) {
		return fmt.Errorf("inventory file %s not found", inventoryFile)
	}
	if _, err := os.Stat(playbookFile); os.IsNotExist(err) {
		return fmt.Errorf("playbook file %s not found", playbookFile)
	}

	fmt.Println("--> Executing ansible-playbook on target-server...")
	cmd := exec.CommandContext(ctx, ansibleBin,
		"-i", inventoryFile,
		playbookFile,
		"--extra-vars", fmt.Sprintf("@%s", tempFile),
		"--limit", ContainerName,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ansible-playbook execution failed: %w", err)
	}

	return nil
}

// VerifyApp checks health and expected version on http://<host>:<port>/
func VerifyApp(ctx context.Context, host string, port int, expectedVersion string) error {
	healthURL := fmt.Sprintf("http://%s:%d/health", host, port)
	rootURL := fmt.Sprintf("http://%s:%d/", host, port)

	fmt.Println("============================================================")
	fmt.Printf(" Verifying Dewy deployment on %s:%d\n", host, port)
	fmt.Printf(" Expected Version: %s\n", expectedVersion)
	fmt.Println("============================================================")

	client := &http.Client{Timeout: 3 * time.Second}
	maxAttempts := 30
	sleepSec := 2 * time.Second

	// 1. Health check
	fmt.Printf("--> Checking health endpoint (%s)...\n", healthURL)
	healthOK := false
	for i := 1; i <= maxAttempts; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		resp, err := client.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.Contains(string(body), `"status":"ok"`) {
				fmt.Printf("[OK] Health check passed on attempt %d.\n", i)
				healthOK = true
				break
			}
		}
		fmt.Printf("    Attempt %d/%d waiting for Dewy service to become healthy...\n", i, maxAttempts)
		time.Sleep(sleepSec)
	}

	if !healthOK {
		return fmt.Errorf("health check timed out after %d attempts", maxAttempts)
	}

	// 2. Version check
	fmt.Printf("--> Checking application version endpoint (%s)...\n", rootURL)
	req, _ := http.NewRequestWithContext(ctx, "GET", rootURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request root endpoint: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	bodyStr := string(bodyBytes)

	fmt.Println("\n--- HTTP Response Content ---")
	fmt.Println(bodyStr)
	fmt.Println("-----------------------------")

	cleanExpected := strings.TrimPrefix(expectedVersion, "v")
	if strings.Contains(bodyStr, expectedVersion) || strings.Contains(bodyStr, cleanExpected) {
		fmt.Printf("[OK] Version verified successfully! (Matches %s)\n", expectedVersion)
		return nil
	}

	return fmt.Errorf("version mismatch! Expected version '%s' not found in response", expectedVersion)
}

// ShowLogs streams Dewy systemd service logs from target-server
func ShowLogs(ctx context.Context, lines int) error {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found in PATH")
	}

	if lines <= 0 {
		lines = 50
	}

	fmt.Printf("--> Fetching last %d lines of Dewy logs from %s...\n", lines, ContainerName)
	cmd := exec.CommandContext(ctx, dockerBin, "exec", ContainerName,
		"journalctl", "-u", "dewy-binary", "-n", fmt.Sprintf("%d", lines), "--no-pager")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Clean stops and removes the target-server container
func Clean(ctx context.Context) error {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found in PATH")
	}

	fmt.Printf("--> Stopping and removing container %s...\n", ContainerName)
	cmd := exec.CommandContext(ctx, dockerBin, "rm", "-f", ContainerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
