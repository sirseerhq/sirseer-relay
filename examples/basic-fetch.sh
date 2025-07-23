#!/bin/bash
# basic-fetch.sh - Basic examples of fetching pull requests

set -e

# Check for required environment variable
if [ -z "$GITHUB_TOKEN" ]; then
    echo "Error: GITHUB_TOKEN environment variable is not set"
    echo "Export your GitHub token: export GITHUB_TOKEN=ghp_your_token"
    exit 1
fi

echo "=== Basic Fetch Examples ==="
echo

# Example 1: Fetch first page of PRs (default behavior)
echo "1. Fetching first page of PRs from golang/go..."
sirseer-relay fetch golang/go --output golang-first-page.ndjson
echo "   Saved to: golang-first-page.ndjson"
echo "   Count: $(wc -l < golang-first-page.ndjson) PRs"
echo

# Example 2: Fetch all PRs from a small repository
echo "2. Fetching all PRs from octocat/Hello-World..."
sirseer-relay fetch octocat/Hello-World --all --output hello-world-all.ndjson
echo "   Saved to: hello-world-all.ndjson"
echo "   Count: $(wc -l < hello-world-all.ndjson) PRs"
echo

# Example 3: Fetch to stdout and process with jq
echo "3. Fetching and processing PRs in real-time..."
echo "   Open PRs in golang/go:"
sirseer-relay fetch golang/go | jq -r 'select(.state == "open") | .title' | head -5
echo

# Example 4: Save with custom timeout for large repos
echo "4. Fetching with extended timeout..."
sirseer-relay fetch kubernetes/kubernetes --request-timeout 600 --output k8s-sample.ndjson
echo "   Saved first page to: k8s-sample.ndjson"
echo

# Example 5: Using token via command line (not recommended for scripts)
echo "5. Fetch with explicit token (example only)..."
# sirseer-relay fetch owner/repo --token $GITHUB_TOKEN

echo "=== Basic fetch examples completed ==="