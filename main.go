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

package main

import (
	"fmt"
	"os"

	"github.com/sh0jitmy/dewyctl/internal/cmd"
)

var Version = "0.1.0"

func printUsage() {
	fmt.Printf("dewyctl v%s - Dewy Deployment & Operations Toolkit\n\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  dewyctl                     Launch interactive operations menu")
	fmt.Println("  dewyctl <command> [options] Run specific command directly")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  test    Run automated E2E zero-downtime deployment test (v1 -> run Dewy -> v2 -> verify)")
	fmt.Println("  server  Launch Dewy server daemon on the current host (auto-fetches dewy if needed)")
	fmt.Println("  release Run GoReleaser with credentials automatically injected from SOPS")
	fmt.Println("  push    Cross-compile Go application and upload binary archive to Object Storage")
	fmt.Println("  config  Interactively configure S3 credentials & bucket, encrypt with SOPS, and test access")
	fmt.Println("  init    Generate GoReleaser (.goreleaser.yaml), tagpr Actions, systemd service, and starter code")
	fmt.Println("  docker  Manage local Docker verification environment (setup, verify, logs, clean, all)")
	fmt.Println("  doctor  Diagnose environment, configuration files, and S3 connectivity")
	fmt.Println("  menu    Open the interactive operations menu")
	fmt.Println("  sops    Manage secret encryption with SOPS and age (config, init, decrypt, edit)")
	fmt.Println("  ansible Generate Ansible playbooks and roles for Dewy")
	fmt.Println("  version Show dewyctl version")
	fmt.Println("  help    Show this help message")
	fmt.Println("\nRun 'dewyctl <command> -h' for more information about a command.")
}

func main() {
	if len(os.Args) < 2 {
		// Launch interactive operations menu by default
		if err := cmd.RunMenu(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "menu":
		err = cmd.RunMenu()
	case "test", "e2e":
		err = cmd.RunTest(args)
	case "server", "run":
		err = cmd.RunServer(args)
	case "release":
		err = cmd.RunRelease(args)
	case "config", "configure":
		err = cmd.RunSopsConfig(args)
	case "init":
		err = cmd.RunInit(args)
	case "docker":
		err = cmd.RunDocker(args)
	case "sops":
		err = cmd.RunSops(args)
	case "ansible":
		err = cmd.RunAnsible(args)
	case "push":
		err = cmd.RunPush(args)
	case "doctor":
		err = cmd.RunDoctor(args)
	case "version", "-v", "--version":
		fmt.Printf("dewyctl version %s\n", Version)
		return
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
