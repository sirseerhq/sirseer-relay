# SirSeer Relay Usage Guide

This comprehensive guide covers all features and usage patterns for sirseer-relay.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Basic Usage](#basic-usage)
3. [Fetching All Pull Requests](#fetching-all-pull-requests)
4. [Time Window Filtering](#time-window-filtering)
5. [Incremental Fetching](#incremental-fetching)
6. [Output Options](#output-options)
7. [Configuration Files](#configuration-files)
8. [Performance Tuning](#performance-tuning)
9. [Enterprise Configuration](#enterprise-configuration)
10. [Exit Codes](#exit-codes)

## Prerequisites

Before using sirseer-relay, ensure you have:

1. **GitHub Personal Access Token** with `repo` scope
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

2. **Repository Access** - Your token must have read access to the target repository

## Basic Usage

The simplest usage fetches the most recent 50 pull requests:

```bash
sirseer-relay fetch owner/repository
```

Example:
```bash
sirseer-relay fetch golang/go
```

This outputs NDJSON (Newline Delimited JSON) to stdout, with one pull request per line.

## Fetching All Pull Requests

To fetch the complete history of pull requests from a repository:

```bash
sirseer-relay fetch owner/repository --all
```

This will:
- Display real-time progress with ETA
- Automatically handle pagination
- Stream results to avoid memory accumulation
- Recover from API complexity errors

Example with file output:
```bash
sirseer-relay fetch kubernetes/kubernetes --all --output k8s-prs.ndjson
```

Progress indicator example:
```
Fetching all 12543 pull requests from kubernetes/kubernetes...
Progress: 3250 / 12543 PRs [25.9%] | Page 65 | ETA: 2m15s
```

## Time Window Filtering

Filter pull requests by creation date using `--since` and `--until` flags.

### Date Formats

sirseer-relay supports multiple date formats:

1. **ISO Date Format (YYYY-MM-DD)**
   ```bash
   sirseer-relay fetch golang/go --since 2024-01-01 --until 2024-12-31
   ```

2. **RFC3339 Format**
   ```bash
   sirseer-relay fetch golang/go --since 2024-01-01T00:00:00Z
   ```

3. **Relative Dates**
   ```bash
   # PRs from last 7 days
   sirseer-relay fetch golang/go --since 7d
   
   # PRs from last 30 days
   sirseer-relay fetch golang/go --since 30d
   ```

### Common Time Window Examples

**Fetch Q1 2024 PRs:**
```bash
sirseer-relay fetch owner/repo --since 2024-01-01 --until 2024-03-31 --all
```

**Fetch PRs from the last month:**
```bash
sirseer-relay fetch owner/repo --since 30d --all
```

**Fetch PRs created in a specific month:**
```bash
sirseer-relay fetch owner/repo --since 2024-06-01 --until 2024-06-30 --all
```

**Fetch all PRs since a specific date:**
```bash
sirseer-relay fetch owner/repo --since 2024-01-01 --all
```

### Important Notes

- Date filters apply to PR **creation date**, not update date
- All dates are interpreted in UTC
- When using `--until` alone, it fetches from the beginning up to that date
- Date ranges are inclusive on both ends

## Incremental Fetching

Incremental fetching allows you to efficiently update your dataset by fetching only new pull requests since the last run.

### Initial Setup

First, perform a full fetch to establish the baseline:

```bash
sirseer-relay fetch owner/repo --all --output baseline.ndjson
```

This creates a state file at `~/.sirseer/state/owner-repo.state` that tracks:
- Last PR number fetched
- Last PR creation date
- Total PRs fetched
- Fetch completion time

### Subsequent Updates

Use `--incremental` to fetch only new PRs:

```bash
sirseer-relay fetch owner/repo --incremental --output updates.ndjson
```

This will:
1. Load the previous state
2. Fetch only PRs created after the last fetch
3. Use PR numbers for deduplication
4. Update the state file upon completion

### Combining Incremental with Time Windows

You can combine incremental fetching with date filters:

```bash
# Fetch new PRs but only up to a specific date
sirseer-relay fetch owner/repo --incremental --until 2024-12-31
```

### Incremental Fetch Workflow

**Daily Update Script:**
```bash
#!/bin/bash
REPO="kubernetes/kubernetes"
DATE=$(date +%Y%m%d)
OUTPUT="k8s-prs-${DATE}.ndjson"

# Fetch new PRs since last run
sirseer-relay fetch $REPO --incremental --output $OUTPUT

# Append to master file
cat $OUTPUT >> k8s-prs-master.ndjson
```

### State File Management

If you encounter issues with incremental fetching:

1. **Check state file location:**
   ```bash
   ls ~/.sirseer/state/
   ```

2. **Reset state (perform full fetch again):**
   ```bash
   rm ~/.sirseer/state/owner-repo.state
   sirseer-relay fetch owner/repo --all
   ```

For more details on state management, see [STATE_MANAGEMENT.md](STATE_MANAGEMENT.md).

## Output Options

### Standard Output (Default)

By default, sirseer-relay writes to stdout:

```bash
sirseer-relay fetch owner/repo | jq '.title'
```

### File Output

Use `--output` to write to a file:

```bash
sirseer-relay fetch owner/repo --all --output prs.ndjson
```

### Processing Output

The NDJSON format is ideal for streaming processing:

**Count PRs by state:**
```bash
cat prs.ndjson | jq -r '.state' | sort | uniq -c
```

**Extract PR numbers and titles:**
```bash
cat prs.ndjson | jq -r '[.number, .title] | @csv'
```

**Filter merged PRs:**
```bash
cat prs.ndjson | jq 'select(.merged_at != null)'
```

## Configuration Files

sirseer-relay supports YAML configuration files for advanced settings and customization.

### Configuration File Search Order

The tool looks for configuration files in the following order:
1. Path specified with `--config` flag
2. `.sirseer-relay.yaml` in the current directory
3. `~/.sirseer/config.yaml` (global configuration)

### Basic Configuration Example

Create `~/.sirseer/config.yaml`:

```yaml
# Default settings for all fetches
defaults:
  batch_size: 25
  state_dir: ~/my-data/sirseer-state

# Repository-specific settings
repositories:
  "kubernetes/kubernetes":
    batch_size: 10  # Large PRs, use smaller batches
```

### Configuration Precedence

Settings are applied in this order (highest precedence first):
1. Command-line flags
2. Environment variables
3. Repository-specific configuration
4. Global configuration file
5. Built-in defaults

### Available Configuration Options

- **github.api_endpoint**: GitHub API base URL
- **github.graphql_endpoint**: GitHub GraphQL endpoint
- **github.token_env**: Environment variable name for token (default: GITHUB_TOKEN)
- **defaults.batch_size**: PRs per API call (1-100)
- **defaults.output_format**: Output format (currently only "ndjson")
- **defaults.state_dir**: Directory for state files
- **repositories**: Map of repo-specific overrides
- **rate_limit.auto_wait**: Auto-wait on rate limit
- **rate_limit.show_progress**: Show progress while waiting

### Environment Variable Overrides

You can override any configuration setting using environment variables:

```bash
# Override batch size
export SIRSEER_BATCH_SIZE=30

# Override state directory
export SIRSEER_STATE_DIR=/custom/state

# Override GitHub endpoints (for Enterprise)
export GITHUB_API_ENDPOINT=https://github.company.com/api/v3
export GITHUB_GRAPHQL_ENDPOINT=https://github.company.com/api/graphql
```

### Using Custom Config Files

Specify a custom configuration file:

```bash
sirseer-relay --config /path/to/custom.yaml fetch org/repo --all
```

For a complete example with all available options, see [`.sirseer-relay.example.yaml`](../.sirseer-relay.example.yaml) in the project root.

## Performance Tuning

### Request Timeout

For large repositories or slow connections, increase the timeout:

```bash
sirseer-relay fetch owner/repo --all --request-timeout 300
```

### Handling Large Repositories

sirseer-relay automatically handles:
- **Memory efficiency**: Uses < 100MB regardless of repository size
- **Query complexity**: Automatically reduces batch size if needed
- **Rate limiting**: Respects GitHub's rate limits

### Network Considerations

For unstable connections:
1. Use `--incremental` to resume if interrupted
2. Increase `--request-timeout` for slow networks
3. The tool will retry on transient failures

## Enterprise Configuration

For GitHub Enterprise installations, create `~/.sirseer/config.yaml`:

```yaml
github:
  api_endpoint: https://github.your-company.com/api/v3
  graphql_endpoint: https://github.your-company.com/api/graphql
```

Then use sirseer-relay normally:

```bash
sirseer-relay fetch org/repo --all
```

## Exit Codes

sirseer-relay uses specific exit codes for different error conditions:

| Code | Meaning | Action |
|------|---------|--------|
| 0 | Success | No action needed |
| 1 | General error | Check error message |
| 2 | Authentication error | Verify GitHub token |
| 3 | Network error | Check connection |

Example error handling in scripts:

```bash
#!/bin/bash
sirseer-relay fetch owner/repo --all --output prs.ndjson

case $? in
    0)
        echo "Success!"
        ;;
    2)
        echo "Authentication failed. Check your GITHUB_TOKEN"
        exit 1
        ;;
    3)
        echo "Network error. Retrying..."
        sleep 60
        exec $0
        ;;
    *)
        echo "Unknown error occurred"
        exit 1
        ;;
esac
```

## Next Steps

- Learn about [State Management](STATE_MANAGEMENT.md) for incremental fetching
- See [Troubleshooting](TROUBLESHOOTING.md) for common issues
- Review [Examples](../examples/) for automation scripts