#!/bin/bash
# Weekly full repository scan script for sirseer-relay
# This script performs a complete fetch of all configured repositories

set -euo pipefail

# Configuration
SIRSEER_BIN="${SIRSEER_BIN:-/usr/local/bin/sirseer-relay}"
CONFIG_FILE="${SIRSEER_CONFIG:-$HOME/.config/sirseer-relay/config.yaml}"
LOG_DIR="${SIRSEER_LOG_DIR:-/var/log/sirseer-relay}"
ARCHIVE_DIR="${SIRSEER_ARCHIVE_DIR:-/var/data/sirseer-relay/weekly}"
REPOS_FILE="${SIRSEER_REPOS:-$HOME/.config/sirseer-relay/repositories.txt}"

# Ensure directories exist
mkdir -p "$LOG_DIR" "$ARCHIVE_DIR"

# Log file with date
LOG_FILE="$LOG_DIR/weekly-$(date +%Y%m%d).log"

# Function to log messages
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# Function to fetch repository with full scan
fetch_repo_full() {
    local repo="$1"
    local output_file="$ARCHIVE_DIR/${repo//\//_}-$(date +%Y%m%d).ndjson"
    
    log "Starting full fetch for $repo"
    
    if $SIRSEER_BIN fetch "$repo" \
        --all \
        --output "$output_file" \
        --config "$CONFIG_FILE" \
        2>&1 | tee -a "$LOG_FILE"; then
        
        # Compress the output
        log "Compressing output to ${output_file}.gz"
        gzip "$output_file"
        
        log "Successfully fetched and archived $repo"
        return 0
    else
        log "ERROR: Failed to fetch $repo"
        return 1
    fi
}

# Function to clean old archives
clean_old_archives() {
    log "Cleaning archives older than 30 days"
    find "$ARCHIVE_DIR" -name "*.ndjson.gz" -mtime +30 -delete
}

# Main execution
log "=== Starting weekly full repository scan ==="

# Check prerequisites
if ! command -v "$SIRSEER_BIN" &> /dev/null; then
    log "ERROR: sirseer-relay not found at $SIRSEER_BIN"
    exit 1
fi

if [[ ! -f "$REPOS_FILE" ]]; then
    log "ERROR: Repositories file not found at $REPOS_FILE"
    exit 1
fi

# Track metrics
success_count=0
failure_count=0
start_time=$(date +%s)

# Process each repository
while IFS= read -r repo || [[ -n "$repo" ]]; do
    # Skip empty lines and comments
    [[ -z "$repo" || "$repo" =~ ^[[:space:]]*# ]] && continue
    
    if fetch_repo_full "$repo"; then
        ((success_count++))
    else
        ((failure_count++))
    fi
done < "$REPOS_FILE"

# Clean old archives
clean_old_archives

# Calculate duration
end_time=$(date +%s)
duration=$((end_time - start_time))
hours=$((duration / 3600))
minutes=$(((duration % 3600) / 60))

# Summary
log "=== Weekly scan complete ==="
log "Duration: ${hours}h ${minutes}m"
log "Successful: $success_count repositories"
log "Failed: $failure_count repositories"
log "Archives stored in: $ARCHIVE_DIR"

# Send notification if configured
if [[ -n "${SLACK_WEBHOOK_URL:-}" ]]; then
    curl -X POST "$SLACK_WEBHOOK_URL" \
        -H "Content-Type: application/json" \
        -d "{\"text\":\"Weekly SirSeer scan complete: $success_count succeeded, $failure_count failed\"}"
fi

# Exit with error if any repos failed
[[ $failure_count -eq 0 ]] || exit 1