#!/bin/bash
# Multi-repository fetch script for sirseer-relay
# Fetch multiple repositories with proper error handling and reporting

set -euo pipefail

# Configuration
SIRSEER_BIN="${SIRSEER_BIN:-sirseer-relay}"
OUTPUT_DIR="${SIRSEER_OUTPUT_DIR:-./data}"
MODE="${SIRSEER_MODE:-incremental}" # 'incremental' or 'all'

# Function to display usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS] repo1 repo2 ... repoN

Fetch multiple GitHub repositories using sirseer-relay.

OPTIONS:
    -m, --mode MODE        Fetch mode: 'incremental' or 'all' (default: incremental)
    -o, --output DIR       Output directory (default: ./data)
    -c, --config FILE      Config file path
    -p, --parallel NUM     Number of parallel fetches (default: 1)
    -h, --help            Show this help message

EXAMPLES:
    # Fetch three repositories incrementally
    $0 kubernetes/kubernetes prometheus/prometheus grafana/grafana

    # Fetch all PRs from multiple repos in parallel
    $0 --mode all --parallel 3 org/repo1 org/repo2 org/repo3

    # Use custom output directory and config
    $0 --output /data/github --config prod.yaml org/repo

ENVIRONMENT VARIABLES:
    GITHUB_TOKEN          Required: GitHub personal access token
    SIRSEER_BIN          Path to sirseer-relay binary
    SIRSEER_OUTPUT_DIR   Default output directory
    SIRSEER_MODE         Default fetch mode
EOF
}

# Parse command line arguments
REPOS=()
PARALLEL=1
CONFIG_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--mode)
            MODE="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -p|--parallel)
            PARALLEL="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo "Unknown option: $1" >&2
            usage
            exit 1
            ;;
        *)
            REPOS+=("$1")
            shift
            ;;
    esac
done

# Validate inputs
if [[ ${#REPOS[@]} -eq 0 ]]; then
    echo "Error: No repositories specified" >&2
    usage
    exit 1
fi

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "Error: GITHUB_TOKEN environment variable not set" >&2
    exit 1
fi

if ! command -v "$SIRSEER_BIN" &> /dev/null; then
    echo "Error: sirseer-relay not found at: $SIRSEER_BIN" >&2
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Function to fetch a single repository
fetch_repo() {
    local repo="$1"
    local output_file="$OUTPUT_DIR/${repo//\//_}.ndjson"
    local status_file="$OUTPUT_DIR/.${repo//\//_}.status"
    
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Fetching $repo..."
    
    # Build command
    local cmd=("$SIRSEER_BIN" "fetch" "$repo" "--output" "$output_file")
    
    # Add mode flag
    if [[ "$MODE" == "all" ]]; then
        cmd+=("--all")
    else
        cmd+=("--incremental")
    fi
    
    # Add config if specified
    if [[ -n "$CONFIG_FILE" ]]; then
        cmd+=("--config" "$CONFIG_FILE")
    fi
    
    # Execute and capture result
    if "${cmd[@]}" 2>&1 | tee "$status_file"; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] ✓ Successfully fetched $repo"
        echo "SUCCESS" >> "$status_file"
        return 0
    else
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] ✗ Failed to fetch $repo" >&2
        echo "FAILED" >> "$status_file"
        return 1
    fi
}

# Export function for parallel execution
export -f fetch_repo
export SIRSEER_BIN OUTPUT_DIR MODE CONFIG_FILE

# Main execution
echo "=== Starting multi-repository fetch ==="
echo "Mode: $MODE"
echo "Output directory: $OUTPUT_DIR"
echo "Parallel jobs: $PARALLEL"
echo "Repositories: ${#REPOS[@]}"
echo

start_time=$(date +%s)

# Execute fetches
if [[ $PARALLEL -eq 1 ]]; then
    # Sequential execution
    success_count=0
    failure_count=0
    
    for repo in "${REPOS[@]}"; do
        if fetch_repo "$repo"; then
            ((success_count++))
        else
            ((failure_count++))
        fi
    done
else
    # Parallel execution using xargs
    printf '%s\n' "${REPOS[@]}" | \
        xargs -P "$PARALLEL" -I {} bash -c 'fetch_repo "$@"' _ {}
    
    # Count results
    success_count=$(grep -l "SUCCESS" "$OUTPUT_DIR"/.*.status 2>/dev/null | wc -l)
    failure_count=$(grep -l "FAILED" "$OUTPUT_DIR"/.*.status 2>/dev/null | wc -l)
fi

# Clean up status files
rm -f "$OUTPUT_DIR"/.*.status

# Calculate duration
end_time=$(date +%s)
duration=$((end_time - start_time))

# Summary report
echo
echo "=== Fetch Summary ==="
echo "Duration: $((duration / 60))m $((duration % 60))s"
echo "Successful: $success_count"
echo "Failed: $failure_count"
echo "Output files in: $OUTPUT_DIR"

# List output files
echo
echo "Generated files:"
find "$OUTPUT_DIR" -name "*.ndjson" -type f -exec ls -lh {} \;

# Exit with error if any repos failed
[[ $failure_count -eq 0 ]] || exit 1