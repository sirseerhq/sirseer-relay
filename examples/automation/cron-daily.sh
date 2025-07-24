#!/bin/bash
# Daily incremental fetch script for sirseer-relay
# This script performs incremental updates for configured repositories

set -euo pipefail

# Configuration
SIRSEER_BIN="${SIRSEER_BIN:-/usr/local/bin/sirseer-relay}"
CONFIG_FILE="${SIRSEER_CONFIG:-$HOME/.config/sirseer-relay/config.yaml}"
LOG_DIR="${SIRSEER_LOG_DIR:-/var/log/sirseer-relay}"
STATE_DIR="${SIRSEER_STATE_DIR:-$HOME/.sirseer-relay}"
REPOS_FILE="${SIRSEER_REPOS:-$HOME/.config/sirseer-relay/repositories.txt}"

# Ensure directories exist
mkdir -p "$LOG_DIR" "$STATE_DIR"

# Log file with date
LOG_FILE="$LOG_DIR/daily-$(date +%Y%m%d).log"

# Function to log messages
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# Function to fetch a repository
fetch_repo() {
    local repo="$1"
    local output_file="${repo//\//_}.ndjson"
    
    log "Starting incremental fetch for $repo"
    
    if $SIRSEER_BIN fetch "$repo" \
        --incremental \
        --output "$output_file" \
        --config "$CONFIG_FILE" \
        2>&1 | tee -a "$LOG_FILE"; then
        log "Successfully fetched $repo"
        return 0
    else
        log "ERROR: Failed to fetch $repo"
        return 1
    fi
}

# Main execution
log "=== Starting daily incremental fetch ==="

# Check if sirseer-relay is available
if ! command -v "$SIRSEER_BIN" &> /dev/null; then
    log "ERROR: sirseer-relay not found at $SIRSEER_BIN"
    exit 1
fi

# Check if repositories file exists
if [[ ! -f "$REPOS_FILE" ]]; then
    log "ERROR: Repositories file not found at $REPOS_FILE"
    log "Create this file with one repository per line (e.g., 'owner/repo')"
    exit 1
fi

# Track success/failure
success_count=0
failure_count=0

# Process each repository
while IFS= read -r repo || [[ -n "$repo" ]]; do
    # Skip empty lines and comments
    [[ -z "$repo" || "$repo" =~ ^[[:space:]]*# ]] && continue
    
    if fetch_repo "$repo"; then
        ((success_count++))
    else
        ((failure_count++))
    fi
done < "$REPOS_FILE"

# Summary
log "=== Daily fetch complete ==="
log "Successful: $success_count repositories"
log "Failed: $failure_count repositories"

# Exit with error if any repos failed
[[ $failure_count -eq 0 ]] || exit 1