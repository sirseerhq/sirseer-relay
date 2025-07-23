# SirSeer Relay

A high-performance tool for extracting pull request metadata from GitHub repositories.

## Overview

SirSeer Relay is a command-line tool that efficiently extracts comprehensive pull request data from GitHub repositories. It's designed to handle repositories of any size while maintaining low memory usage through streaming architecture.

## Features

- **Efficient Data Extraction**: Fetches complete PR metadata including reviews, commits, and files
- **Full Repository Fetching**: Extract all pull requests with automatic pagination
- **Constant Memory Usage**: Streams data directly to disk, using less than 100MB regardless of repository size
- **Live Progress Tracking**: Real-time progress indicator with percentage completion and ETA
- **Automatic Query Recovery**: Intelligently handles GraphQL complexity limits by adjusting batch size
- **Incremental Updates**: Resume from where you left off with built-in state management
- **Enterprise Ready**: Supports GitHub Enterprise installations
- **Cross-Platform**: Available for Linux, macOS, and Windows

## Requirements

- Go 1.21 or later (for building from source)
- GitHub personal access token with repository read permissions

## Installation

### From Source

```bash
git clone https://github.com/sirseerhq/sirseer-relay.git
cd sirseer-relay
make build
```

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/sirseerhq/sirseer-relay/releases) page.

## Usage

### Basic Usage

By default, sirseer-relay fetches only the first page of pull requests (up to 50 most recently updated PRs):

```bash
export GITHUB_TOKEN=your_github_token
./sirseer-relay fetch organization/repository
```

### Fetching All Pull Requests

To fetch all pull requests from a repository with automatic pagination:

```bash
./sirseer-relay fetch organization/repository --all
```

This will display a real-time progress indicator:
```
Fetching all 12543 pull requests from kubernetes/kubernetes...
Progress: 3250 / 12543 PRs [25.9%] | Page 65 | ETA: 2m15s
```

### Command Syntax

```
sirseer-relay fetch <org>/<repo> [flags]
```

### Options

- `--token`: GitHub personal access token (overrides GITHUB_TOKEN env var)
- `--output`: Output file path (default: stdout)
- `--all`: Fetch all pull requests from the repository
- `--request-timeout`: Request timeout in seconds (default: 180, useful for large repos)
- `--since`: Fetch PRs created after a specific date (not implemented yet)
- `--until`: Fetch PRs created before a specific date (not implemented yet)
- `--incremental`: Resume from the last successful fetch (not implemented yet)

### Output

Data is exported in NDJSON (Newline Delimited JSON) format, with one pull request per line. This format is ideal for streaming processing and data analysis tools.

## Configuration

For GitHub Enterprise installations, create a configuration file at `~/.sirseer/config.yaml`:

```yaml
github:
  api_endpoint: https://github.your-company.com/api/v3
  graphql_endpoint: https://github.your-company.com/api/graphql
```

## Contributing

When contributing to this repository, please note that we use automated quality checks to maintain high standards.

### Setup

After cloning the repository, configure git to use our custom hooks:

```bash
git config core.hooksPath .githooks
```

This enables automatic pre-commit checks that prevent common issues.

### Before Pushing

Run the pre-push validation script to ensure your changes meet all requirements:

```bash
./scripts/pre-push-check.sh
```

This script checks for code quality issues, runs tests, and validates that no internal development references are present.

## License

This software is licensed under the Business Source License 1.1. See the [LICENSE](LICENSE) file for details.

## Support

For questions or issues, please contact: ip@sirseer.com