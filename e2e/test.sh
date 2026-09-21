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

# E2E Test Script for Dewy Deployed Applications
# Usage: ./test.sh <host> <port> <expected_version>

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <host> <port> <expected_version>"
    exit 1
fi

HOST=$1
PORT=$2
EXPECTED_VERSION=$3
MAX_ATTEMPTS=30
SLEEP_SEC=2

echo "========================================="
echo " Starting E2E Verification for port ${PORT}"
echo " Host: ${HOST}"
echo " Expected Version: ${EXPECTED_VERSION}"
echo "========================================="

# 1. Verify health endpoint
echo "Checking health endpoint (http://${HOST}:${PORT}/health)..."
health_ok=false
for ((i=1; i<=MAX_ATTEMPTS; i++)); do
    response=$(curl -s -f "http://${HOST}:${PORT}/health" || true)
    if [ "${response}" = '{"status":"ok"}' ]; then
        echo "Health check passed on attempt ${i}."
        health_ok=true
        break
    fi
    echo "Attempt ${i}/${MAX_ATTEMPTS} failed. Retrying in ${SLEEP_SEC}s..."
    sleep ${SLEEP_SEC}
done

if [ "${health_ok}" = false ]; then
    echo "Error: Health check timed out or failed!"
    exit 1
fi

# 2. Verify root page and version
echo "Checking version endpoint (http://${HOST}:${PORT}/)..."
version_ok=false
response_body=$(curl -s -f "http://${HOST}:${PORT}/")

echo "--- Response Content ---"
echo "${response_body}"
echo "------------------------"

if echo "${response_body}" | grep -q "Version: ${EXPECTED_VERSION}"; then
    echo "Version match verified successfully!"
    version_ok=true
else
    echo "Error: Version mismatch! Expected: ${EXPECTED_VERSION}"
    exit 1
fi

echo "E2E Test PASSED for port ${PORT}."
echo "========================================="
