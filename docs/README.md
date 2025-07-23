# SirSeer Relay Documentation

Welcome to the comprehensive documentation for sirseer-relay.

## Quick Links

- **[Usage Guide](USAGE.md)** - Complete guide to all features and options
- **[State Management](STATE_MANAGEMENT.md)** - Understanding incremental fetch and state files  
- **[Troubleshooting](TROUBLESHOOTING.md)** - Solutions to common issues

## Documentation Overview

### For New Users

1. Start with the main [README](../README.md) for installation
2. Read the [Usage Guide](USAGE.md) for detailed examples
3. Check [Troubleshooting](TROUBLESHOOTING.md) if you encounter issues

### For Advanced Users

1. Learn about [State Management](STATE_MANAGEMENT.md) for incremental fetching
2. Review [Examples](../examples/) for automation scripts
3. Understand the [NDJSON output format](#output-format)

## Key Concepts

### Time Window Filtering

Filter pull requests by creation date:
- Use `--since` and `--until` flags
- Supports multiple date formats (YYYY-MM-DD, RFC3339, relative)
- Enables efficient historical data extraction

### Incremental Fetching

Update your dataset efficiently:
- First run: Full fetch with `--all`
- Subsequent runs: Only new PRs with `--incremental`
- Automatic deduplication using PR numbers

### State Management

Tracks fetch progress automatically:
- State files in `~/.sirseer/state/`
- Atomic writes prevent corruption
- Checksum validation ensures integrity

## Output Format

SirSeer Relay outputs NDJSON (Newline Delimited JSON) with the following schema:

```json
{
  "number": 12345,
  "title": "Fix memory leak in parser",
  "state": "closed",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-16T14:20:00Z",
  "closed_at": "2024-01-16T14:20:00Z",
  "merged_at": "2024-01-16T14:20:00Z",
  "author": {
    "login": "developer123"
  }
}
```

Each line is a complete JSON object representing one pull request.

## Performance Characteristics

- **Memory Usage**: < 100MB regardless of repository size
- **Streaming**: No in-memory accumulation
- **Automatic Recovery**: Handles API complexity limits
- **Progress Tracking**: Real-time ETA calculation

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/sirseerhq/sirseer-relay/issues)
- **Email**: ip@sirseer.com
- **Examples**: See the [examples directory](../examples/)