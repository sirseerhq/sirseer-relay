#!/bin/bash
# incremental-fetch.sh - Demonstrates incremental fetching workflow

set -e

# Configuration
REPO="golang/go"  # Change to your repository
OUTPUT_DIR="./pr-data"
DATE=$(date +%Y%m%d-%H%M%S)

# Check prerequisites
if [ -z "$GITHUB_TOKEN" ]; then
    echo "Error: GITHUB_TOKEN not set"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo "=== Incremental Fetch Workflow ==="
echo "Repository: $REPO"
echo "Output directory: $OUTPUT_DIR"
echo

# Function to check if state exists
check_state() {
    local repo_slug=$(echo "$1" | sed 's/\//-/g')
    local state_file="$HOME/.sirseer/state/${repo_slug}.state"
    
    if [ -f "$state_file" ]; then
        echo "State file exists: $state_file"
        return 0
    else
        echo "No state file found"
        return 1
    fi
}

# Step 1: Check if this is first run
if ! check_state "$REPO"; then
    echo "First run detected. Performing initial full fetch..."
    echo "This may take a while for large repositories..."
    
    sirseer-relay fetch "$REPO" --all --output "$OUTPUT_DIR/${REPO//\//-}-initial.ndjson"
    
    echo "Initial fetch complete!"
    echo "State saved for future incremental updates"
    echo
fi

# Step 2: Perform incremental fetch
echo "Performing incremental fetch..."
INCREMENTAL_FILE="$OUTPUT_DIR/${REPO//\//-}-increment-${DATE}.ndjson"

sirseer-relay fetch "$REPO" --incremental --output "$INCREMENTAL_FILE"

# Check if any new PRs were found
if [ -s "$INCREMENTAL_FILE" ]; then
    NEW_COUNT=$(wc -l < "$INCREMENTAL_FILE")
    echo "Found $NEW_COUNT new PRs"
    echo "Saved to: $INCREMENTAL_FILE"
    
    # Show summary of new PRs
    echo
    echo "New PR Summary:"
    jq -r '[.number, .title, .author.login] | @csv' "$INCREMENTAL_FILE" | head -10
    
    # Append to master file (optional)
    MASTER_FILE="$OUTPUT_DIR/${REPO//\//-}-master.ndjson"
    cat "$INCREMENTAL_FILE" >> "$MASTER_FILE"
    echo
    echo "Appended to master file: $MASTER_FILE"
else
    echo "No new PRs found since last fetch"
    rm -f "$INCREMENTAL_FILE"  # Clean up empty file
fi

echo
echo "=== Incremental fetch completed ==="

# Show state info
echo
echo "State Information:"
cat ~/.sirseer/state/"${REPO//\//-}".state | jq .