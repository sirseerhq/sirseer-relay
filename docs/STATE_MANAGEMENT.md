# State Management in SirSeer Relay

This document explains how sirseer-relay tracks fetch progress and enables incremental updates through its state management system.

## Table of Contents

1. [Overview](#overview)
2. [Why State Management?](#why-state-management)
3. [State File Location](#state-file-location)
4. [State File Schema](#state-file-schema)
5. [How It Works](#how-it-works)
6. [Recovery Procedures](#recovery-procedures)
7. [Best Practices](#best-practices)
8. [Troubleshooting](#troubleshooting)

## Overview

SirSeer Relay uses a state management system to:
- Track the progress of repository fetches
- Enable incremental updates (fetch only new PRs)
- Provide resilience against interruptions
- Maintain data integrity with checksums

## Why State Management?

Without state management, you would need to:
1. Re-fetch all PRs every time (wasteful and slow)
2. Manually track what was fetched last
3. Risk missing PRs or fetching duplicates

The state system solves these problems automatically.

## State File Location

State files are stored in a standardized location:

```
~/.sirseer/state/<org>-<repo>.state
```

Examples:
- `~/.sirseer/state/golang-go.state`
- `~/.sirseer/state/kubernetes-kubernetes.state`
- `~/.sirseer/state/facebook-react.state`

To view your state files:
```bash
ls -la ~/.sirseer/state/
```

## State File Schema

State files are JSON formatted for easy inspection and debugging:

```json
{
  "version": 1,
  "checksum": "a3f5e9c2b8d4f6e1a9c3b7d5e8f2a4c6b9d1e3f5",
  "repository": "kubernetes/kubernetes",
  "last_fetch_id": "full-1704067200",
  "last_pr_number": 123456,
  "last_pr_date": "2024-01-15T10:30:00Z",
  "last_fetch_time": "2024-01-15T14:45:00Z",
  "total_fetched": 12543
}
```

### Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `version` | int | Schema version for future compatibility |
| `checksum` | string | SHA256 hash for integrity validation |
| `repository` | string | Full repository name (org/repo) |
| `last_fetch_id` | string | Unique identifier for the fetch operation |
| `last_pr_number` | int | Highest PR number seen in the last fetch |
| `last_pr_date` | time | Creation date of the newest PR fetched |
| `last_fetch_time` | time | When the fetch completed successfully |
| `total_fetched` | int | Total number of PRs fetched in that operation |

## How It Works

### 1. Initial Full Fetch

When you run a full fetch:

```bash
sirseer-relay fetch owner/repo --all
```

The process:
1. Fetches all PRs from the repository
2. Tracks the highest PR number and latest creation date
3. Saves state upon successful completion
4. State file is written atomically (no corruption risk)

### 2. Incremental Fetch

When you run with `--incremental`:

```bash
sirseer-relay fetch owner/repo --incremental
```

The process:
1. Loads the previous state file
2. Queries for PRs created after `last_pr_date`
3. Filters out any PRs with number ≤ `last_pr_number` (deduplication)
4. Writes only new PRs to output
5. Updates state file with new progress

### 3. Atomic Writes

State files are written atomically to prevent corruption:

1. New state is written to a temporary file (`.state.tmp`)
2. Checksum is calculated and included
3. File is synced to disk
4. Atomic rename replaces the old state file

This ensures the state file is never left in a partial or corrupted state.

## Recovery Procedures

### Corrupted State File

**Symptoms:**
```
Error: State file is corrupted (checksum mismatch).
To recover: Delete '~/.sirseer/state/owner-repo.state' and run again.
```

**Recovery:**
```bash
# Remove corrupted state
rm ~/.sirseer/state/owner-repo.state

# Run a full fetch to rebuild state
sirseer-relay fetch owner/repo --all
```

**Note:** Your previous output files are safe. Only the state tracking is affected.

### Incompatible Version

**Symptoms:**
```
Error: State file version (0) is incompatible with current version (1).
```

**Recovery:**
```bash
# Remove old version state
rm ~/.sirseer/state/owner-repo.state

# Run a full fetch with new version
sirseer-relay fetch owner/repo --all
```

### Missing State File

**Symptoms:**
```
Error: No previous fetch state found for owner/repo.
To start an incremental fetch, first run a full fetch without --incremental.
```

**Recovery:**
```bash
# Perform initial full fetch
sirseer-relay fetch owner/repo --all

# Now incremental fetches will work
sirseer-relay fetch owner/repo --incremental
```

### Repository Mismatch

**Symptoms:**
```
Error: State file is for repository org1/repo1 but current command is for org2/repo2
```

**Recovery:**
This is a safety check. Each repository has its own state file. Ensure you're using the correct repository name.

## Best Practices

### 1. Regular Incremental Updates

Set up a cron job for daily updates:

```bash
# crontab -e
0 2 * * * /usr/local/bin/sirseer-relay fetch owner/repo --incremental --output /data/prs-$(date +\%Y\%m\%d).ndjson
```

### 2. State Backup

For critical workflows, backup state files:

```bash
# Before major operations
cp ~/.sirseer/state/owner-repo.state ~/.sirseer/state/owner-repo.state.backup

# Restore if needed
cp ~/.sirseer/state/owner-repo.state.backup ~/.sirseer/state/owner-repo.state
```

### 3. Monitoring State Health

Check state file integrity:

```bash
# View state file
cat ~/.sirseer/state/owner-repo.state | jq .

# Check last fetch time
cat ~/.sirseer/state/owner-repo.state | jq -r .last_fetch_time

# Verify checksum (the tool does this automatically)
cat ~/.sirseer/state/owner-repo.state | jq -r 'del(.checksum)' | sha256sum
```

### 4. Handling Repository Renames

If a repository is renamed:

1. The old state file will no longer match
2. You'll need to run a full fetch with the new name
3. Optionally, rename the state file:

```bash
mv ~/.sirseer/state/old-name.state ~/.sirseer/state/new-name.state
# Edit the file to update the "repository" field
```

## Troubleshooting

### Q: Can I edit the state file manually?

**A:** While technically possible, it's not recommended because:
- The checksum will become invalid
- You might miss PRs or get duplicates
- It's safer to delete and re-fetch

### Q: What happens if a fetch is interrupted?

**A:** The state file is only updated after successful completion. If interrupted:
- The previous state remains intact
- You can safely re-run the same command
- No PRs will be duplicated

### Q: How do I reset everything and start fresh?

**A:** Remove all state files:
```bash
rm -rf ~/.sirseer/state/
```

### Q: Can I use the same state across multiple machines?

**A:** Yes, state files are portable. You can:
1. Copy the state file to another machine
2. Place it in the correct location
3. Continue incremental fetches

### Q: How much disk space do state files use?

**A:** State files are tiny (typically < 1KB each). Even with 1000 repositories, total space would be under 1MB.

## Advanced Usage

### Custom State File Location

While not recommended, you can manually manage state:

```bash
# Save current state
cp ~/.sirseer/state/owner-repo.state ./my-custom-state.json

# Later, restore it
cp ./my-custom-state.json ~/.sirseer/state/owner-repo.state
```

### State File Validation Script

Create a script to validate state files:

```bash
#!/bin/bash
# validate-state.sh

STATE_FILE="$1"
if [ -z "$STATE_FILE" ]; then
    echo "Usage: $0 <state-file>"
    exit 1
fi

# Check if file exists
if [ ! -f "$STATE_FILE" ]; then
    echo "Error: State file not found"
    exit 1
fi

# Validate JSON
if ! jq . "$STATE_FILE" > /dev/null 2>&1; then
    echo "Error: Invalid JSON"
    exit 1
fi

# Check required fields
for field in version checksum repository last_pr_number; do
    if ! jq -e ".$field" "$STATE_FILE" > /dev/null 2>&1; then
        echo "Error: Missing required field: $field"
        exit 1
    fi
done

echo "State file is valid"
```

## See Also

- [Usage Guide](USAGE.md) - General usage instructions
- [Troubleshooting](TROUBLESHOOTING.md) - Common issues and solutions
- [Examples](../examples/) - Automation scripts using state management