#!/bin/bash
# daily-update.sh - Automated daily update script for cron jobs

set -e

# Configuration
REPOS=(
    "kubernetes/kubernetes"
    "golang/go"
    "docker/docker"
)

OUTPUT_BASE="/var/data/github-prs"  # Change to your data directory
LOG_DIR="/var/log/sirseer-relay"
DATE=$(date +%Y%m%d)
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Ensure directories exist
mkdir -p "$OUTPUT_BASE" "$LOG_DIR"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_DIR/daily-update-$DATE.log"
}

# Error handling
error_exit() {
    log "ERROR: $1"
    exit 1
}

# Check prerequisites
if [ -z "$GITHUB_TOKEN" ]; then
    error_exit "GITHUB_TOKEN environment variable not set"
fi

# Check if sirseer-relay is available
if ! command -v sirseer-relay &> /dev/null; then
    error_exit "sirseer-relay not found in PATH"
fi

log "=== Starting daily update run ==="
log "Repositories: ${REPOS[*]}"

# Track statistics
TOTAL_NEW_PRS=0
FAILED_REPOS=()

# Process each repository
for REPO in "${REPOS[@]}"; do
    log "Processing $REPO..."
    
    REPO_SLUG="${REPO//\//-}"
    OUTPUT_DIR="$OUTPUT_BASE/$REPO_SLUG"
    mkdir -p "$OUTPUT_DIR"
    
    # Check if initial fetch has been done
    STATE_FILE="$HOME/.sirseer/state/${REPO_SLUG}.state"
    if [ ! -f "$STATE_FILE" ]; then
        log "  No state file found. Performing initial full fetch..."
        
        if sirseer-relay fetch "$REPO" --all \
            --output "$OUTPUT_DIR/initial-$TIMESTAMP.ndjson" \
            2>&1 | tee -a "$LOG_DIR/daily-update-$DATE.log"; then
            
            log "  Initial fetch completed successfully"
        else
            log "  ERROR: Initial fetch failed for $REPO"
            FAILED_REPOS+=("$REPO")
            continue
        fi
    fi
    
    # Perform incremental fetch
    INCREMENT_FILE="$OUTPUT_DIR/increment-$DATE.ndjson"
    
    if sirseer-relay fetch "$REPO" --incremental \
        --output "$INCREMENT_FILE" \
        2>&1 | tee -a "$LOG_DIR/daily-update-$DATE.log"; then
        
        if [ -s "$INCREMENT_FILE" ]; then
            NEW_COUNT=$(wc -l < "$INCREMENT_FILE")
            log "  Found $NEW_COUNT new PRs"
            TOTAL_NEW_PRS=$((TOTAL_NEW_PRS + NEW_COUNT))
            
            # Append to monthly master file
            MONTH=$(date +%Y%m)
            MASTER_FILE="$OUTPUT_DIR/master-$MONTH.ndjson"
            cat "$INCREMENT_FILE" >> "$MASTER_FILE"
            
            # Generate daily summary
            {
                echo "Repository: $REPO"
                echo "Date: $DATE"
                echo "New PRs: $NEW_COUNT"
                echo "Top Authors:"
                jq -r '.author.login' "$INCREMENT_FILE" | sort | uniq -c | sort -nr | head -5
                echo
            } > "$OUTPUT_DIR/summary-$DATE.txt"
            
        else
            log "  No new PRs found"
            rm -f "$INCREMENT_FILE"  # Clean up empty file
        fi
    else
        log "  ERROR: Incremental fetch failed for $REPO"
        FAILED_REPOS+=("$REPO")
    fi
    
    log "  Completed $REPO"
    echo
done

# Summary report
log "=== Daily update completed ==="
log "Total new PRs across all repos: $TOTAL_NEW_PRS"

if [ ${#FAILED_REPOS[@]} -gt 0 ]; then
    log "Failed repositories: ${FAILED_REPOS[*]}"
    
    # Send alert (implement your alerting mechanism)
    # echo "SirSeer Relay: Failed to update ${#FAILED_REPOS[@]} repositories" | mail -s "Daily Update Failed" admin@company.com
    
    exit 1
else
    log "All repositories updated successfully"
fi

# Cleanup old increment files (keep 30 days)
find "$OUTPUT_BASE" -name "increment-*.ndjson" -mtime +30 -delete

# Compress old monthly files (keep current and previous month uncompressed)
CURRENT_MONTH=$(date +%Y%m)
PREVIOUS_MONTH=$(date -d "1 month ago" +%Y%m)

find "$OUTPUT_BASE" -name "master-*.ndjson" -type f | while read -r file; do
    filename=$(basename "$file")
    month="${filename#master-}"
    month="${month%.ndjson}"
    
    if [ "$month" != "$CURRENT_MONTH" ] && [ "$month" != "$PREVIOUS_MONTH" ]; then
        if [ ! -f "$file.gz" ]; then
            log "Compressing old monthly file: $file"
            gzip -9 "$file"
        fi
    fi
done

log "Cleanup completed"
exit 0