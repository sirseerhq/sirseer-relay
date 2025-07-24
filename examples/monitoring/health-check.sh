#!/bin/bash
# Health check script for sirseer-relay operations
# Can be used with monitoring systems or as a standalone check

set -euo pipefail

# Configuration
METADATA_DIR="${SIRSEER_METADATA_DIR:-./metadata}"
STATE_DIR="${SIRSEER_STATE_DIR:-$HOME/.sirseer-relay}"
MAX_AGE_HOURS="${SIRSEER_MAX_AGE_HOURS:-24}"
REPOS_FILE="${SIRSEER_REPOS:-$HOME/.config/sirseer-relay/repositories.txt}"

# Exit codes
EXIT_OK=0
EXIT_WARNING=1
EXIT_CRITICAL=2
EXIT_UNKNOWN=3

# Track status
overall_status=$EXIT_OK
messages=()

# Function to add message and update status
add_message() {
    local level="$1"
    local message="$2"
    
    messages+=("[$level] $message")
    
    case "$level" in
        CRITICAL)
            [[ $overall_status -lt $EXIT_CRITICAL ]] && overall_status=$EXIT_CRITICAL
            ;;
        WARNING)
            [[ $overall_status -lt $EXIT_WARNING ]] && overall_status=$EXIT_WARNING
            ;;
    esac
}

# Function to check metadata freshness
check_metadata_freshness() {
    local repo="$1"
    local metadata_file="$METADATA_DIR/${repo//\//_}_metadata.json"
    
    if [[ ! -f "$metadata_file" ]]; then
        add_message "WARNING" "$repo: No metadata file found"
        return
    fi
    
    # Get last fetch time
    local end_time
    end_time=$(jq -r '.endTime // empty' "$metadata_file" 2>/dev/null)
    
    if [[ -z "$end_time" ]]; then
        add_message "WARNING" "$repo: No endTime in metadata"
        return
    fi
    
    # Convert to epoch
    local fetch_epoch
    fetch_epoch=$(date -d "$end_time" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%S" "${end_time%.*}" +%s 2>/dev/null)
    
    if [[ -z "$fetch_epoch" ]]; then
        add_message "WARNING" "$repo: Cannot parse endTime"
        return
    fi
    
    # Check age
    local current_epoch=$(date +%s)
    local age_hours=$(( (current_epoch - fetch_epoch) / 3600 ))
    
    if [[ $age_hours -gt $MAX_AGE_HOURS ]]; then
        add_message "WARNING" "$repo: Last fetch ${age_hours}h ago (max: ${MAX_AGE_HOURS}h)"
    fi
    
    # Check for errors
    local error
    error=$(jq -r '.error // empty' "$metadata_file" 2>/dev/null)
    
    if [[ -n "$error" ]]; then
        add_message "CRITICAL" "$repo: Last fetch failed: $error"
    fi
}

# Function to check state file integrity
check_state_integrity() {
    local repo="$1"
    local state_file="$STATE_DIR/${repo//\//_}.state"
    
    if [[ ! -f "$state_file" ]]; then
        # Not critical - might be first run
        return
    fi
    
    # Verify JSON validity
    if ! jq empty "$state_file" 2>/dev/null; then
        add_message "CRITICAL" "$repo: State file corrupted"
        return
    fi
    
    # Check checksum if present
    local stored_checksum
    stored_checksum=$(jq -r '.checksum // empty' "$state_file" 2>/dev/null)
    
    if [[ -n "$stored_checksum" ]]; then
        # Recalculate checksum
        local calculated_checksum
        calculated_checksum=$(jq 'del(.checksum)' "$state_file" | sha256sum | cut -d' ' -f1)
        
        if [[ "$stored_checksum" != "$calculated_checksum" ]]; then
            add_message "CRITICAL" "$repo: State file checksum mismatch"
        fi
    fi
}

# Function to check disk space
check_disk_space() {
    local dir="$1"
    local threshold=90
    
    if [[ ! -d "$dir" ]]; then
        return
    fi
    
    local usage
    usage=$(df -P "$dir" | awk 'NR==2 {print $5}' | sed 's/%//')
    
    if [[ $usage -gt $threshold ]]; then
        add_message "WARNING" "Disk usage for $dir at ${usage}% (threshold: ${threshold}%)"
    fi
}

# Main checks
echo "=== SirSeer Relay Health Check ==="
echo "Metadata directory: $METADATA_DIR"
echo "State directory: $STATE_DIR"
echo "Max age: ${MAX_AGE_HOURS}h"
echo

# Check if directories exist
if [[ ! -d "$METADATA_DIR" ]]; then
    add_message "CRITICAL" "Metadata directory does not exist: $METADATA_DIR"
fi

if [[ ! -d "$STATE_DIR" ]]; then
    add_message "WARNING" "State directory does not exist: $STATE_DIR"
fi

# Check disk space
check_disk_space "$METADATA_DIR"
check_disk_space "$STATE_DIR"

# Check each repository
if [[ -f "$REPOS_FILE" ]]; then
    while IFS= read -r repo || [[ -n "$repo" ]]; do
        # Skip empty lines and comments
        [[ -z "$repo" || "$repo" =~ ^[[:space:]]*# ]] && continue
        
        check_metadata_freshness "$repo"
        check_state_integrity "$repo"
    done < "$REPOS_FILE"
else
    add_message "WARNING" "Repositories file not found: $REPOS_FILE"
fi

# Output results
echo "Status: $(case $overall_status in
    0) echo "OK" ;;
    1) echo "WARNING" ;;
    2) echo "CRITICAL" ;;
    *) echo "UNKNOWN" ;;
esac)"

# Print all messages
for msg in "${messages[@]}"; do
    echo "$msg"
done

# Performance data for monitoring systems
echo
echo "Performance Data:"
echo "- Repositories checked: $(grep -v '^#' "$REPOS_FILE" 2>/dev/null | grep -c . || echo 0)"
echo "- Metadata files: $(find "$METADATA_DIR" -name "*_metadata.json" 2>/dev/null | wc -l)"
echo "- State files: $(find "$STATE_DIR" -name "*.state" 2>/dev/null | wc -l)"

exit $overall_status