# SirSeer Relay Examples

This directory contains example scripts demonstrating common usage patterns for sirseer-relay.

## Available Examples

- **[basic-fetch.sh](basic-fetch.sh)** - Basic fetching operations
- **[incremental-fetch.sh](incremental-fetch.sh)** - Setting up incremental updates
- **[time-window-fetch.sh](time-window-fetch.sh)** - Date filtering examples
- **[daily-update.sh](daily-update.sh)** - Automated daily update script

## Running Examples

All scripts are bash scripts that can be run directly:

```bash
# Make executable
chmod +x examples/*.sh

# Run a script
./examples/basic-fetch.sh
```

## Prerequisites

- `sirseer-relay` installed and in PATH
- `GITHUB_TOKEN` environment variable set
- `jq` installed (for JSON processing examples)

## Customization

These scripts are starting points. Modify them for your specific needs:

- Change repository names
- Adjust date ranges
- Add error handling
- Integrate with your data pipeline