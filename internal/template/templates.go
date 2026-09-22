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

package template

import "strings"

// Config represents parameters supplied to templates
type Config struct {
	AppName    string
	Port       string
	S3Bucket   string
	S3Prefix   string
	S3Endpoint string
	S3Region   string
	BinaryDir  string
	GitOwner   string
	GitRepo    string
}

// SystemdServiceTemplate returns a systemd service unit file for Dewy binary deployment.
func SystemdServiceTemplate(cfg Config) string {
	prefix := cfg.S3Prefix
	if prefix == "" {
		prefix = "sample-app"
	}
	tmpl := `[Unit]
Description=Dewy Binary Deployment Service ({{APP_NAME}})
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory={{BINARY_DIR}}
EnvironmentFile=/etc/dewy-binary.env
ExecStart=/usr/local/bin/dewy server --registry 's3://{{S3_REGION}}/{{S3_BUCKET}}/{{S3_PREFIX}}?endpoint={{S3_ENDPOINT}}&artifact={{APP_NAME}}_linux_amd64.tar.gz' --port {{PORT}} -- {{BINARY_DIR}}/current/{{APP_NAME}}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
	r := strings.NewReplacer(
		"{{APP_NAME}}", cfg.AppName,
		"{{BINARY_DIR}}", cfg.BinaryDir,
		"{{S3_REGION}}", cfg.S3Region,
		"{{S3_BUCKET}}", cfg.S3Bucket,
		"{{S3_PREFIX}}", prefix,
		"{{S3_ENDPOINT}}", cfg.S3Endpoint,
		"{{PORT}}", cfg.Port,
	)
	return r.Replace(tmpl)
}

// EnvFileTemplate returns the environment variable template for Dewy
func EnvFileTemplate(cfg Config) string {
	tmpl := `# AWS / S3-compatible object storage credentials for Dewy
AWS_ACCESS_KEY_ID=your_access_key_here
AWS_SECRET_ACCESS_KEY=your_secret_key_here
AWS_DEFAULT_REGION={{S3_REGION}}
GITHUB_TOKEN=your_optional_github_token_here
`
	return strings.ReplaceAll(tmpl, "{{S3_REGION}}", cfg.S3Region)
}

// GoReleaserTemplate returns a .goreleaser.yaml configured for Dewy S3 registry
func GoReleaserTemplate(cfg Config) string {
	tmpl := `version: 2

project_name: {{APP_NAME}}

before:
  hooks:
    - go mod tidy

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    main: .
    binary: {{APP_NAME}}
    ldflags:
      - -s -w -X main.Version={{.Version}}

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    files:
      - none*

# Upload archives to Object Storage (S3 compatible)
blobs:
  - provider: s3
    bucket: "{{ .Env.S3_BUCKET }}"
    region: "{{ .Env.S3_REGION }}"
    endpoint: "{{ .Env.S3_ENDPOINT }}"
    # Dewy S3 registry path convention: <prefix>/<semver>/<artifact>
    directory: "{{ if .Env.S3_PREFIX }}{{ .Env.S3_PREFIX }}{{ else }}sample-app{{ end }}/{{ .Tag }}"
    ids:
      - default
`
	return strings.ReplaceAll(tmpl, "{{APP_NAME}}", cfg.AppName)
}

// TagprWorkflowTemplate returns the GitHub Actions workflow for tagpr + S3 PUT
func TagprWorkflowTemplate(cfg Config) string {
	tmpl := `name: tagpr and Release to Object Storage

on:
  push:
    branches:
      - main

permissions:
  contents: write
  pull-requests: write

jobs:
  tagpr:
    name: Manage Release PR & Tag
    runs-on: ubuntu-latest
    outputs:
      tag: ${{ steps.tagpr.outputs.tag }}
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Run tagpr
        id: tagpr
        uses: Songmu/tagpr@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  release:
    name: Build & Upload to Object Storage
    needs: tagpr
    if: needs.tagpr.outputs.tag != ''
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Fetch tags
        run: git fetch --force --tags

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
          cache: true

      - name: Run GoReleaser (Upload binary to S3)
        uses: goreleaser/goreleaser-action@v5
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          S3_BUCKET: ${{ secrets.S3_BUCKET }}
          S3_REGION: {{S3_REGION}}
          S3_ENDPOINT: {{S3_ENDPOINT}}
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
`
	r := strings.NewReplacer(
		"{{S3_REGION}}", cfg.S3Region,
		"{{S3_ENDPOINT}}", cfg.S3Endpoint,
	)
	return r.Replace(tmpl)
}

// StarterGoSnippet returns a Go code boilerplate for server-starter socket inheritance
func StarterGoSnippet() string {
	return `package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lestrrat-go/server-starter/listener"
)

var Version = "dev"

func main() {
	portFlag := flag.String("port", "8080", "Fallback port if not under server-starter")
	flag.Parse()

	// 1. Retrieve listener from server-starter (Dewy inherits listening socket)
	listeners, err := listener.ListenAll()
	if err != nil && err != listener.ErrNoListeningTarget {
		log.Fatalf("Failed to initialize server-starter listener: %v", err)
	}

	var l net.Listener
	if len(listeners) > 0 {
		log.Println("Running under server-starter. Inheriting listening socket.")
		l = listeners[0]
	} else {
		addr := ":" + *portFlag
		log.Printf("No server-starter listener detected. Listening directly on %s", addr)
		l, err = net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("Failed to listen on %s: %v", addr, err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from Dewy Application! (Version: %s)\n", Version)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{\"status\":\"ok\"}"))
	})

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 2. Handle Graceful Shutdown (server-starter sends SIGTERM to old process)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}
	log.Println("Server stopped.")
}
`
}
