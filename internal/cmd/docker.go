package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sh0jitmy/dewyctl/internal/docker"
	"github.com/sh0jitmy/dewyctl/internal/sops"
)

func printDockerUsage() {
	fmt.Println("Usage: dewyctl docker <subcommand> [options]")
	fmt.Println("\nSubcommands:")
	fmt.Println("  setup   Start target-server container and deploy Dewy via Ansible")
	fmt.Println("  verify  Verify application health and version on local container (:8080)")
	fmt.Println("  logs    View Dewy systemd service logs inside container")
	fmt.Println("  clean   Stop and remove target-server container")
	fmt.Println("  all     Run all-in-one local verification: setup -> push -> verify")
	fmt.Println("\nRun 'dewyctl docker <subcommand> -h' for subcommand options.")
}

// RunDocker routes docker subcommands
func RunDocker(args []string) error {
	if len(args) == 0 {
		printDockerUsage()
		return nil
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "setup":
		return runDockerSetup(subArgs)
	case "verify":
		return runDockerVerify(subArgs)
	case "logs":
		return runDockerLogs(subArgs)
	case "clean":
		return runDockerClean(subArgs)
	case "all":
		return runDockerAll(subArgs)
	case "help", "-h", "--help":
		printDockerUsage()
		return nil
	default:
		return fmt.Errorf("unknown docker subcommand: %s (run 'dewyctl docker help' for usage)", sub)
	}
}

func checkCredentialsBeforeSetup() error {
	keyFile := "key.txt"
	encFile := "secrets.enc.yml"
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return fmt.Errorf("key file %s not found. Please run 'dewyctl config' to set up credentials", keyFile)
	}
	if _, err := os.Stat(encFile); os.IsNotExist(err) {
		return fmt.Errorf("secrets file %s not found. Please run 'dewyctl config' to set up credentials", encFile)
	}
	decrypted, err := sops.Decrypt(keyFile, encFile)
	if err != nil {
		return fmt.Errorf("failed to decrypt %s: %w", encFile, err)
	}
	if strings.Contains(decrypted, "placeholder") {
		fmt.Println("============================================================")
		fmt.Println(" [!] Warning: secrets.enc.yml contains placeholders.")
		fmt.Println("     Dewy needs real Sakura Cloud S3 credentials to poll.")
		fmt.Println("     Please run 'dewyctl config' if you have not configured them.")
		fmt.Println("============================================================")
	}
	return nil
}

func runDockerSetup(args []string) error {
	if err := checkCredentialsBeforeSetup(); err != nil {
		return err
	}

	ctx := context.Background()

	fmt.Println("============================================================")
	fmt.Println(" [dewyctl docker setup] Provisioning Dewy on Local Container")
	fmt.Println("============================================================")

	// 1. Ensure target-server container is running
	if err := docker.EnsureContainer(ctx); err != nil {
		return err
	}

	// 2. Run Ansible playbook against container
	if err := docker.RunAnsible(ctx); err != nil {
		return err
	}

	// 3. Output container IP and instructions
	ip, err := docker.GetContainerIP(ctx)
	if err != nil {
		return err
	}

	fmt.Println("\n============================================================")
	fmt.Println(" [+] Dewy setup completed successfully in Docker container!")
	fmt.Printf(" [+] Target Container IP: %s\n", ip)
	fmt.Println(" [+] Dewy daemon is active and polling Object Storage.")
	fmt.Println(" Next step:")
	fmt.Println("   1. Run 'dewyctl push --version v0.1.0' to upload your binary")
	fmt.Println("   2. Run 'dewyctl docker verify --version v0.1.0' to test")
	fmt.Println("============================================================")
	return nil
}

func runDockerVerify(args []string) error {
	fs := flag.NewFlagSet("docker verify", flag.ContinueOnError)
	version := fs.String("version", "v0.1.0", "Expected application version")
	port := fs.Int("port", 8080, "Target application port")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	ctx := context.Background()
	ip, err := docker.GetContainerIP(ctx)
	if err != nil {
		return fmt.Errorf("could not get container IP. Is target-server running? Run 'dewyctl docker setup' first: %w", err)
	}

	return docker.VerifyApp(ctx, ip, *port, *version)
}

func runDockerLogs(args []string) error {
	fs := flag.NewFlagSet("docker logs", flag.ContinueOnError)
	lines := fs.Int("lines", 50, "Number of log lines to show")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	ctx := context.Background()
	return docker.ShowLogs(ctx, *lines)
}

func runDockerClean(args []string) error {
	ctx := context.Background()
	return docker.Clean(ctx)
}

func runDockerAll(args []string) error {
	fs := flag.NewFlagSet("docker all", flag.ContinueOnError)
	version := fs.String("version", "v0.1.0", "Release version to deploy and verify")
	port := fs.Int("port", 8080, "Application port")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	fmt.Println("============================================================")
	fmt.Printf(" [dewyctl docker all] All-in-One Local Verification (%s)\n", *version)
	fmt.Println("============================================================")

	// Step 1: Setup container and Ansible
	fmt.Println("\n[Step 1/3] Setting up Docker container and running Ansible...")
	if err := runDockerSetup(nil); err != nil {
		return fmt.Errorf("step 1 failed: %w", err)
	}

	// Step 2: Push binary to S3
	fmt.Printf("\n[Step 2/3] Building and pushing binary to Object Storage (%s)...\n", *version)
	if err := RunPush([]string{"--version", *version}); err != nil {
		return fmt.Errorf("step 2 (push) failed: %w", err)
	}

	// Step 3: Wait for Dewy polling & verify
	fmt.Println("\n[Step 3/3] Waiting for Dewy to auto-detect release, then verifying...")
	time.Sleep(5 * time.Second)

	ctx := context.Background()
	ip, err := docker.GetContainerIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container IP: %w", err)
	}

	if err := docker.VerifyApp(ctx, ip, *port, *version); err != nil {
		return fmt.Errorf("step 3 (verify) failed: %w", err)
	}

	fmt.Println("\n============================================================")
	fmt.Println(" [+] All-in-One Local Docker Verification PASSED!")
	fmt.Printf(" [+] App is successfully running on http://%s:%d\n", ip, *port)
	fmt.Println("============================================================")
	return nil
}
