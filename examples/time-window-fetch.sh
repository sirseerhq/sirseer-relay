#!/bin/bash
# time-window-fetch.sh - Examples of time-based filtering

set -e

# Check prerequisites
if [ -z "$GITHUB_TOKEN" ]; then
    echo "Error: GITHUB_TOKEN not set"
    exit 1
fi

REPO="golang/go"  # Change to your repository
OUTPUT_DIR="./time-filtered"
mkdir -p "$OUTPUT_DIR"

echo "=== Time Window Filtering Examples ==="
echo "Repository: $REPO"
echo

# Example 1: Fetch PRs from specific year
echo "1. Fetching PRs created in 2023..."
sirseer-relay fetch "$REPO" \
    --since 2023-01-01 \
    --until 2023-12-31 \
    --all \
    --output "$OUTPUT_DIR/prs-2023.ndjson"

if [ -s "$OUTPUT_DIR/prs-2023.ndjson" ]; then
    echo "   Found $(wc -l < "$OUTPUT_DIR/prs-2023.ndjson") PRs from 2023"
fi
echo

# Example 2: Fetch PRs from last 30 days
echo "2. Fetching PRs from last 30 days..."
sirseer-relay fetch "$REPO" \
    --since 30d \
    --all \
    --output "$OUTPUT_DIR/prs-last-30d.ndjson"

if [ -s "$OUTPUT_DIR/prs-last-30d.ndjson" ]; then
    echo "   Found $(wc -l < "$OUTPUT_DIR/prs-last-30d.ndjson") PRs from last 30 days"
fi
echo

# Example 3: Fetch PRs from specific quarter
echo "3. Fetching Q1 2024 PRs..."
sirseer-relay fetch "$REPO" \
    --since 2024-01-01 \
    --until 2024-03-31 \
    --all \
    --output "$OUTPUT_DIR/prs-2024-q1.ndjson"

if [ -s "$OUTPUT_DIR/prs-2024-q1.ndjson" ]; then
    echo "   Found $(wc -l < "$OUTPUT_DIR/prs-2024-q1.ndjson") PRs from Q1 2024"
fi
echo

# Example 4: Fetch PRs from specific month
YEAR=$(date +%Y)
MONTH=$(date +%m)
LAST_DAY=$(date -d "$YEAR-$MONTH-01 +1 month -1 day" +%d 2>/dev/null || echo "31")

echo "4. Fetching PRs from current month ($YEAR-$MONTH)..."
sirseer-relay fetch "$REPO" \
    --since "$YEAR-$MONTH-01" \
    --until "$YEAR-$MONTH-$LAST_DAY" \
    --output "$OUTPUT_DIR/prs-current-month.ndjson"
echo

# Example 5: Analysis by time period
echo "5. Analyzing PR creation patterns..."
echo

# Monthly breakdown for a year
echo "Monthly PR count for 2023:"
for month in {01..12}; do
    # Determine last day of month
    if [ "$month" = "02" ]; then
        last_day="28"
    elif [ "$month" = "04" ] || [ "$month" = "06" ] || [ "$month" = "09" ] || [ "$month" = "11" ]; then
        last_day="30"
    else
        last_day="31"
    fi
    
    # Fetch count for the month
    count=$(sirseer-relay fetch "$REPO" \
        --since "2023-$month-01" \
        --until "2023-$month-$last_day" \
        2>/dev/null | wc -l)
    
    printf "   %s 2023: %d PRs\n" "$(date -d "2023-$month-01" +%B 2>/dev/null || echo "Month $month")" "$count"
done

echo
echo "=== Time window examples completed ==="
echo "Output files saved in: $OUTPUT_DIR/"