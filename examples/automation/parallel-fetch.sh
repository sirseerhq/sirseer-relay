#!/bin/bash
# Parallel repository fetching for organizations
# Efficiently fetch all repositories from one or more GitHub organizations

set -euo pipefail

# Configuration
SIRSEER_BIN="${SIRSEER_BIN:-sirseer-relay}"
OUTPUT_DIR="${SIRSEER_OUTPUT_DIR:-./data}"
MAX_PARALLEL="${SIRSEER_MAX_PARALLEL:-5}"
GITHUB_API="${GITHUB_API_URL:-https://api.github.com}"

# Function to display usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS] org1 [org2 ... orgN]

Fetch all repositories from GitHub organizations in parallel.

OPTIONS:
    -o, --output DIR       Output directory (default: ./data)
    -p, --parallel NUM     Max parallel fetches (default: 5)
    -f, --filter PATTERN   Only fetch repos matching pattern (regex)
    -x, --exclude PATTERN  Exclude repos matching pattern (regex)
    -m, --mode MODE       Fetch mode: 'incremental' or 'all' (default: incremental)
    --public-only         Only fetch public repositories
    --archived            Include archived repositories
    -h, --help           Show this help message

EXAMPLES:
    # Fetch all repos from kubernetes org
    $0 kubernetes

    # Fetch from multiple orgs with custom parallelism
    $0 --parallel 10 kubernetes prometheus grafana

    # Filter repositories by name pattern
    $0 --filter "^kube-" kubernetes

    # Exclude test repositories
    $0 --exclude "-test$" myorg

ENVIRONMENT VARIABLES:
    GITHUB_TOKEN          Required: GitHub personal access token
    SIRSEER_BIN          Path to sirseer-relay binary
    SIRSEER_MAX_PARALLEL Default max parallel fetches
EOF
}

# Parse arguments
ORGS=()
FILTER_PATTERN=""
EXCLUDE_PATTERN=""
MODE="incremental"
PUBLIC_ONLY=false
INCLUDE_ARCHIVED=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -p|--parallel)
            MAX_PARALLEL="$2"
            shift 2
            ;;
        -f|--filter)
            FILTER_PATTERN="$2"
            shift 2
            ;;
        -x|--exclude)
            EXCLUDE_PATTERN="$2"
            shift 2
            ;;
        -m|--mode)
            MODE="$2"
            shift 2
            ;;
        --public-only)
            PUBLIC_ONLY=true
            shift
            ;;
        --archived)
            INCLUDE_ARCHIVED=true
            shift
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
            ORGS+=("$1")
            shift
            ;;
    esac
done

# Validate inputs
if [[ ${#ORGS[@]} -eq 0 ]]; then
    echo "Error: No organizations specified" >&2
    usage
    exit 1
fi

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "Error: GITHUB_TOKEN environment variable not set" >&2
    exit 1
fi

# Function to list repositories for an organization
list_org_repos() {
    local org="$1"
    local page=1
    local per_page=100
    
    echo "Discovering repositories for organization: $org" >&2
    
    while true; do
        local url="$GITHUB_API/orgs/$org/repos?per_page=$per_page&page=$page"
        
        if [[ "$PUBLIC_ONLY" == "true" ]]; then
            url="${url}&type=public"
        fi
        
        local response
        response=$(curl -sf -H "Authorization: token $GITHUB_TOKEN" \
                        -H "Accept: application/vnd.github.v3+json" \
                        "$url") || {
            echo "Error: Failed to list repos for $org" >&2
            return 1
        }
        
        # Extract repository names
        local repos
        repos=$(echo "$response" | jq -r '.[] | 
            select(.archived == false or $archived) | 
            .full_name' --argjson archived "$INCLUDE_ARCHIVED")
        
        # Apply filters
        if [[ -n "$FILTER_PATTERN" ]]; then
            repos=$(echo "$repos" | grep -E "$FILTER_PATTERN" || true)
        fi
        
        if [[ -n "$EXCLUDE_PATTERN" ]]; then
            repos=$(echo "$repos" | grep -vE "$EXCLUDE_PATTERN" || true)
        fi
        
        # Output repos
        echo "$repos"
        
        # Check if there are more pages
        local repo_count
        repo_count=$(echo "$response" | jq '. | length')
        
        if [[ $repo_count -lt $per_page ]]; then
            break
        fi
        
        ((page++))
    done
}

# Function to fetch a repository
fetch_repo() {
    local repo="$1"
    local output_file="$OUTPUT_DIR/${repo//\//_}.ndjson"
    local log_file="$OUTPUT_DIR/${repo//\//_}.log"
    
    echo "[$(date '+%H:%M:%S')] Starting: $repo"
    
    local cmd=("$SIRSEER_BIN" "fetch" "$repo" "--output" "$output_file")
    
    if [[ "$MODE" == "all" ]]; then
        cmd+=("--all")
    else
        cmd+=("--incremental")
    fi
    
    if "${cmd[@]}" &> "$log_file"; then
        echo "[$(date '+%H:%M:%S')] ✓ Success: $repo"
        return 0
    else
        echo "[$(date '+%H:%M:%S')] ✗ Failed: $repo (see $log_file)" >&2
        return 1
    fi
}

# Export functions and variables for parallel execution
export -f fetch_repo
export SIRSEER_BIN OUTPUT_DIR MODE

# Main execution
echo "=== Parallel Organization Fetch ==="
echo "Organizations: ${ORGS[*]}"
echo "Max parallel: $MAX_PARALLEL"
echo "Mode: $MODE"
echo "Output directory: $OUTPUT_DIR"
echo

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Collect all repositories
all_repos=()
for org in "${ORGS[@]}"; do
    echo "Discovering repositories in $org..."
    mapfile -t org_repos < <(list_org_repos "$org")
    
    if [[ ${#org_repos[@]} -eq 0 ]]; then
        echo "Warning: No repositories found in $org" >&2
        continue
    fi
    
    echo "Found ${#org_repos[@]} repositories in $org"
    all_repos+=("${org_repos[@]}")
done

# Summary
total_repos=${#all_repos[@]}
echo
echo "Total repositories to fetch: $total_repos"

if [[ $total_repos -eq 0 ]]; then
    echo "No repositories to fetch"
    exit 0
fi

# Confirm if large number
if [[ $total_repos -gt 50 ]]; then
    echo
    read -p "Continue with fetching $total_repos repositories? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted"
        exit 0
    fi
fi

# Start parallel fetch
echo
echo "Starting parallel fetch with $MAX_PARALLEL workers..."
start_time=$(date +%s)

# Use GNU parallel if available, otherwise fall back to xargs
if command -v parallel &> /dev/null; then
    printf '%s\n' "${all_repos[@]}" | \
        parallel -j "$MAX_PARALLEL" --bar fetch_repo {}
else
    printf '%s\n' "${all_repos[@]}" | \
        xargs -P "$MAX_PARALLEL" -I {} bash -c 'fetch_repo "$@"' _ {}
fi

# Calculate statistics
end_time=$(date +%s)
duration=$((end_time - start_time))

success_count=$(find "$OUTPUT_DIR" -name "*.ndjson" -newer "$0" | wc -l)
failure_count=$((total_repos - success_count))

# Summary report
echo
echo "=== Fetch Complete ==="
echo "Duration: $((duration / 60))m $((duration % 60))s"
echo "Total repositories: $total_repos"
echo "Successful: $success_count"
echo "Failed: $failure_count"
echo

# Show largest files
echo "Largest outputs:"
find "$OUTPUT_DIR" -name "*.ndjson" -type f -exec ls -lhS {} \; | head -10

# Exit with error if any failed
[[ $failure_count -eq 0 ]] || exit 1