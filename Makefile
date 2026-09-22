# Makefile for Dewy Deployment Template

.PHONY: help build run test lint vulncheck check pinact pinact-check docker-build sops-encrypt sops-decrypt sops-edit test-local-deploy deploy-prod clean dewyctl dewyctl-test dewyctl-server dewyctl-doctor install

# Configuration
KEY_FILE = key.txt
ENCRYPTED_SECRETS = secrets.enc.yml
DECRYPTED_SECRETS = secrets.yml

help:
	@echo "Available commands:"
	@echo "  build               Build Go application binary"
	@echo "  dewyctl             Build dewyctl CLI tool to bin/dewyctl"
	@echo "  install             Install dewyctl to GOPATH/bin (go install .)"
	@echo "  test                Run all Go unit tests (dewyctl & sample app)"
	@echo "  lint                Run golangci-lint on dewyctl and sample app"
	@echo "  vulncheck           Run govulncheck security analysis"
	@echo "  pinact              Pin GitHub Actions versions with SHA hashes"
	@echo "  pinact-check        Verify that all GitHub Actions are pinned"
	@echo "  check               Run test, lint, vulncheck, and pinact-check all together"
	@echo "  dewyctl-test        Run automated E2E zero-downtime test with dewyctl"
	@echo "  dewyctl-server      Launch Dewy server locally with dewyctl"
	@echo "  dewyctl-doctor      Run dewyctl diagnostic"
	@echo "  run                 Run Go application locally (fallback mode)"
	@echo "  docker-build        Build local Docker image"
	@echo "  sops-encrypt        Encrypt $(DECRYPTED_SECRETS) to $(ENCRYPTED_SECRETS)"
	@echo "  sops-decrypt        Decrypt $(ENCRYPTED_SECRETS) to $(DECRYPTED_SECRETS)"
	@echo "  sops-edit           Edit $(ENCRYPTED_SECRETS) directly in plaintext using SOPS"
	@echo "  test-local-deploy   Simulate deploy and run E2E test locally using Docker"
	@echo "  clean               Clean up build files and decrypted secrets"

build:
	go build -C app -ldflags "-s -w -X main.Version=local-dev" -o dewy-app

dewyctl:
	@mkdir -p bin
	go build -o bin/dewyctl .

install:
	go install .

dewyctl-test: dewyctl
	@./bin/dewyctl test

dewyctl-server: dewyctl
	@./bin/dewyctl server

dewyctl-doctor: dewyctl
	@./bin/dewyctl doctor

run: build
	./app/dewy-app --port 8080

test:
	@echo "==> Running unit tests for dewyctl..."
	go test -v ./...
	@echo "==> Running unit tests for sample app..."
	go test -C app -v ./...

lint:
	@echo "==> Running golangci-lint for dewyctl..."
	@which golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not found. Install via: brew install golangci-lint or go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...
	@echo "==> Running golangci-lint for sample app..."
	(cd app && golangci-lint run ./...)
	@echo "==> Lint check passed!"

vulncheck:
	@echo "==> Running govulncheck for dewyctl..."
	@which govulncheck >/dev/null 2>&1 || (echo "govulncheck not found. Install via: go install golang.org/x/vuln/cmd/govulncheck@latest" && exit 1)
	govulncheck ./...
	@echo "==> Running govulncheck for sample app..."
	govulncheck -C app ./...
	@echo "==> Vulnerability check passed!"

pinact:
	@echo "==> Pinning GitHub Actions versions..."
	@which pinact >/dev/null 2>&1 || (echo "pinact not found. Install via: brew install pinact or go install github.com/suzuki-shunsuke/pinact/cmd/pinact@latest" && exit 1)
	pinact run
	@echo "==> GitHub Actions pinned successfully!"

pinact-check:
	@echo "==> Checking if all GitHub Actions are pinned..."
	@which pinact >/dev/null 2>&1 || (echo "pinact not found. Install via: brew install pinact or go install github.com/suzuki-shunsuke/pinact/cmd/pinact@latest" && exit 1)
	pinact run --check
	@echo "==> All GitHub Actions are properly pinned!"

check: test lint vulncheck pinact-check
	@echo "==> All checks (test, lint, vulncheck, pinact-check) passed!"

docker-build:
	docker build -t ghcr.io/user/dewy-practice:latest .

sops-encrypt:
	@if [ ! -f $(DECRYPTED_SECRETS) ]; then echo "Error: $(DECRYPTED_SECRETS) does not exist."; exit 1; fi
	SOPS_AGE_KEY_FILE=$(KEY_FILE) sops --encrypt $(DECRYPTED_SECRETS) > $(ENCRYPTED_SECRETS)
	@echo "Encrypted successfully to $(ENCRYPTED_SECRETS)"

sops-decrypt:
	@if [ ! -f $(KEY_FILE) ]; then echo "Error: $(KEY_FILE) not found. Put your age private key in $(KEY_FILE)."; exit 1; fi
	SOPS_AGE_KEY_FILE=$(KEY_FILE) sops --decrypt $(ENCRYPTED_SECRETS) > $(DECRYPTED_SECRETS)
	@echo "Decrypted successfully to $(DECRYPTED_SECRETS)"

sops-edit:
	@if [ ! -f $(KEY_FILE) ]; then echo "Error: $(KEY_FILE) not found. Put your age private key in $(KEY_FILE)."; exit 1; fi
	SOPS_AGE_KEY_FILE=$(KEY_FILE) sops $(ENCRYPTED_SECRETS)

test-local-deploy: sops-decrypt
	@echo "==> Starting local deployment simulation in Docker..."
	# Spin up target container if not exists
	@docker inspect target-server >/dev/null 2>&1 || \
		(echo "Spawning target-server systemd container..." && \
		docker run -d --privileged --name target-server \
			-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
			--cgroupns=host \
			-v /var/run/docker.sock:/var/run/docker.sock \
			geerlingguy/docker-ubuntu2204-ansible:latest)
	@sleep 2
	# Run Ansible Playbook (Binary Mode)
	ansible-playbook -i ansible/inventory.ini ansible/playbook-binary.yml --extra-vars @$(DECRYPTED_SECRETS) --limit target-server
	# Run Ansible Playbook (Container Mode)
	ansible-playbook -i ansible/inventory.ini ansible/playbook-container.yml --extra-vars @$(DECRYPTED_SECRETS) --limit target-server
	# Validate deployment
	@TARGET_IP=$$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' target-server); \
	echo "Target container IP: $$TARGET_IP"; \
	./e2e/test.sh "$$TARGET_IP" 8080 "local-dev"; \
	./e2e/test.sh "$$TARGET_IP" 8081 "local-dev"
	@echo "==> Local E2E verification success!"
	@$(MAKE) clean

clean:
	rm -f app/dewy-app
	rm -rf bin
	rm -rf .dewy
	rm -f $(DECRYPTED_SECRETS)
	@docker rm -f target-server >/dev/null 2>&1 || true
