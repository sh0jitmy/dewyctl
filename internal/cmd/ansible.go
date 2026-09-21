package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sh0jitmy/dewyctl/internal/ansible"
)

// RunAnsible executes the ansible generation subcommand
func RunAnsible(args []string) error {
	if len(args) == 0 {
		printAnsibleUsage()
		return nil
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "init":
		return runAnsibleInit(subArgs)
	case "help", "-h", "--help":
		printAnsibleUsage()
		return nil
	default:
		return fmt.Errorf("unknown ansible subcommand: %s (run 'dewyctl ansible help' for usage)", sub)
	}
}

func printAnsibleUsage() {
	fmt.Println("Usage: dewyctl ansible <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  init     Generate complete Ansible playbooks, inventory, and roles for Dewy")
}

func runAnsibleInit(args []string) error {
	fs := flag.NewFlagSet("ansible init", flag.ExitOnError)
	appName := fs.String("app", "", "Application name (default: directory name)")
	binaryPort := fs.String("port", "8080", "Binary HTTP port")
	containerPort := fs.String("container-port", "8081", "Container HTTP port")
	bucket := fs.String("bucket", "my-sakura-bucket", "S3 bucket name")
	endpoint := fs.String("endpoint", "https://s3.tky01.sakurastorage.jp", "S3 endpoint URL")
	region := fs.String("region", "jp-east-1", "S3 region")
	binaryDir := fs.String("dest", "", "Binary destination directory (default: /opt/<app>)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *appName == "" {
		cwd, _ := os.Getwd()
		*appName = filepath.Base(cwd)
	}

	fmt.Println("=== dewyctl ansible: Generate Dewy Ansible Playbooks & Roles ===")

	cfg := ansible.Config{
		AppName:       *appName,
		BinaryPort:    *binaryPort,
		ContainerPort: *containerPort,
		S3Bucket:      *bucket,
		S3Endpoint:    *endpoint,
		S3Region:      *region,
		BinaryDestDir: *binaryDir,
	}

	if err := ansible.GenerateAnsiblePlaybooks(".", cfg); err != nil {
		return fmt.Errorf("failed to generate Ansible structure: %w", err)
	}

	fmt.Println("\nAnsible structure generated successfully!")
	fmt.Println("Usage:")
	fmt.Println("  ansible-playbook -i ansible/inventory.ini ansible/playbook-binary.yml --extra-vars @secrets.yml")
	return nil
}
