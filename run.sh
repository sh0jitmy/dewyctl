#!/usr/bin/env bash
# Copyright (c) 2026 sh0jitmy <shjtmy@gmail.com>
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# ==============================================================================
# run.sh - Dewy Deployment & Operations Launcher
# ==============================================================================
# This script automatically ensures dewyctl is compiled and routes all commands
# to dewyctl. You can run it with no arguments for an interactive menu, or pass
# commands directly (e.g. ./run.sh config, ./run.sh docker all).
# ==============================================================================

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DEWYCTL="${ROOT_DIR}/bin/dewyctl"

# 1. Ensure dewyctl is built and up-to-date
ensure_dewyctl() {
  if [ ! -f "${BIN_DEWYCTL}" ] || [ "${ROOT_DIR}/main.go" -nt "${BIN_DEWYCTL}" ]; then
    echo "==> Building dewyctl CLI..."
    mkdir -p "${ROOT_DIR}/bin"
    go build -C "${ROOT_DIR}" -o "${BIN_DEWYCTL}" .
  fi
}

ensure_dewyctl

# 2. If no arguments are passed, launch the interactive menu
if [ $# -eq 0 ]; then
  exec "${BIN_DEWYCTL}"
fi

# 3. Delegate all commands to dewyctl
exec "${BIN_DEWYCTL}" "$@"
