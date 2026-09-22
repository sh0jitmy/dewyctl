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

package ansible

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds settings for Ansible templates
type Config struct {
	AppName       string
	BinaryPort    string
	ContainerPort string
	S3Bucket      string
	S3Endpoint    string
	S3Region      string
	DewyVersion   string
	BinaryDestDir string
}

// GenerateAnsiblePlaybooks generates complete Ansible structure for Dewy
func GenerateAnsiblePlaybooks(baseDir string, cfg Config) error {
	if cfg.DewyVersion == "" {
		cfg.DewyVersion = "2.14.1"
	}
	if cfg.BinaryDestDir == "" {
		cfg.BinaryDestDir = fmt.Sprintf("/opt/%s", cfg.AppName)
	}
	if cfg.S3Endpoint == "" {
		cfg.S3Endpoint = "https://s3.tky01.sakurastorage.jp"
	}
	if cfg.S3Region == "" {
		cfg.S3Region = "jp-east-1"
	}
	if cfg.ContainerPort == "" {
		cfg.ContainerPort = "8081"
	}

	ansibleDir := filepath.Join(baseDir, "ansible")
	_ = os.MkdirAll(ansibleDir, 0755)

	files := map[string]string{
		// 1. inventory.ini
		filepath.Join(ansibleDir, "inventory.ini"): `[targets]
# Production target resolving IP from the secrets file
my-server ansible_host="{{ deploy_target_ip }}"

# For CI/CD and local docker-based testing
target-server ansible_connection=docker
`,

		// 2. playbook-binary.yml
		filepath.Join(ansibleDir, "playbook-binary.yml"): `---
- name: Deploy app in binary mode using Dewy
  hosts: targets
  become: yes
  roles:
    - dewy
    - dewy-binary
`,

		// 3. group_vars/all/vars.yml
		filepath.Join(ansibleDir, "group_vars", "all", "vars.yml"): fmt.Sprintf(`# Application Details
app_name: "%s"
binary_port: %s
container_port: %s

# Dewy CLI Installation details
dewy_version: "%s"

# S3 / Sakura Object Storage settings (Used for binary mode)
s3_bucket: "%s"
s3_endpoint: "%s"
s3_region: "%s"

# Deployment paths
app_binary_dest_dir: "%s"
`, cfg.AppName, cfg.BinaryPort, cfg.ContainerPort, cfg.DewyVersion, cfg.S3Bucket, cfg.S3Endpoint, cfg.S3Region, cfg.BinaryDestDir),

		// 4. roles/dewy/tasks/main.yml
		filepath.Join(ansibleDir, "roles", "dewy", "tasks", "main.yml"): `---
- name: Set architecture variable
  set_fact:
    dewy_arch: "{{ 'x86_64' if ansible_facts['architecture'] in ['x86_64', 'amd64'] else 'arm64' }}"

- name: Check if Dewy is installed
  stat:
    path: /usr/local/bin/dewy
  register: dewy_bin

- name: Check installed Dewy version
  command: /usr/local/bin/dewy --version
  register: dewy_version_check
  failed_when: false
  changed_when: false
  when: dewy_bin.stat.exists

- name: Install Dewy if not present or version mismatch
  block:
    - name: Create temporary directory
      tempfile:
        state: directory
        suffix: dewy
      register: temp_dir

    - name: Download Dewy archive
      get_url:
        url: "https://github.com/linyows/dewy/releases/download/v{{ dewy_version }}/dewy_linux_{{ dewy_arch }}.tar.gz"
        dest: "{{ temp_dir.path }}/dewy.tar.gz"
        mode: '0644'

    - name: Extract Dewy archive
      unarchive:
        src: "{{ temp_dir.path }}/dewy.tar.gz"
        dest: "{{ temp_dir.path }}"
        remote_src: yes

    - name: Install Dewy binary to /usr/local/bin
      copy:
        src: "{{ temp_dir.path }}/dewy"
        dest: /usr/local/bin/dewy
        mode: '0755'
        remote_src: yes

    - name: Clean up temporary directory
      file:
        path: "{{ temp_dir.path }}"
        state: absent
  when: >
    not dewy_bin.stat.exists or
    dewy_version_check.stdout is not defined or
    dewy_version not in dewy_version_check.stdout
`,

		// 5. roles/dewy-binary/tasks/main.yml
		filepath.Join(ansibleDir, "roles", "dewy-binary", "tasks", "main.yml"): `---
- name: Ensure deployment base directory exists
  file:
    path: "{{ app_binary_dest_dir }}"
    state: directory
    mode: '0755'

- name: Write environment file for Dewy binary mode
  template:
    src: dewy-binary.env.j2
    dest: "/etc/dewy-binary.env"
    mode: '0600'
  notify: Restart dewy-binary

- name: Create systemd unit file for Dewy binary mode
  template:
    src: dewy-binary.service.j2
    dest: "/etc/systemd/system/dewy-binary.service"
    mode: '0644'
  notify: Restart dewy-binary

- name: Force systemd handler to apply config changes
  meta: flush_handlers

- name: Enable and start dewy-binary service
  systemd:
    name: dewy-binary
    enabled: yes
    state: started
`,

		// 6. roles/dewy-binary/templates/dewy-binary.service.j2
		filepath.Join(ansibleDir, "roles", "dewy-binary", "templates", "dewy-binary.service.j2"): `[Unit]
Description=Dewy Binary Deployment Service ({{ app_name }})
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory={{ app_binary_dest_dir }}
EnvironmentFile=/etc/dewy-binary.env
ExecStart=/usr/local/bin/dewy server --registry 's3://{{ s3_region }}/{{ s3_bucket }}/{{ s3_prefix | default("dewyctl") }}?endpoint={{ s3_endpoint }}&artifact={{ app_name }}_linux_{{ dewy_arch }}.tar.gz' --port {{ binary_port }} -- {{ app_binary_dest_dir }}/current/{{ app_name }}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`,

		// 7. roles/dewy-binary/templates/dewy-binary.env.j2
		filepath.Join(ansibleDir, "roles", "dewy-binary", "templates", "dewy-binary.env.j2"): `AWS_ACCESS_KEY_ID={{ aws_access_key_id }}
AWS_SECRET_ACCESS_KEY={{ aws_secret_access_key }}
AWS_DEFAULT_REGION={{ s3_region }}
GITHUB_TOKEN={{ github_token | default('') }}
`,

		// 8. roles/dewy-binary/handlers/main.yml
		filepath.Join(ansibleDir, "roles", "dewy-binary", "handlers", "main.yml"): `---
- name: Restart dewy-binary
  systemd:
    name: dewy-binary
    state: restarted
    daemon_reload: yes
`,
	}

	for path, content := range files {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", path, err)
			}
			fmt.Printf("[+] Created %s\n", path)
		} else {
			fmt.Printf("[!] %s already exists, skipping.\n", path)
		}
	}

	return nil
}
