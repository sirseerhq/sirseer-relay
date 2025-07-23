# SirSeer Relay

A high-performance tool for extracting pull request metadata from GitHub repositories.

## Overview

SirSeer Relay is a command-line tool that efficiently extracts comprehensive pull request data from GitHub repositories. It's designed to handle repositories of any size while maintaining low memory usage through streaming architecture.

## Features

- **Efficient Data Extraction**: Fetches complete PR metadata including reviews, commits, and files
- **Memory Optimized**: Streams data directly to disk, maintaining constant memory usage regardless of repository size
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

To fetch all pull requests from a repository:

```bash
./sirseer-relay fetch organization/repository --all
```

### Command Syntax

```
sirseer-relay fetch <org>/<repo> [flags]
```

### Options

- `--token`: GitHub personal access token (overrides GITHUB_TOKEN env var)
- `--output`: Output file path (default: stdout)
- `--all`: Fetch all pull requests from the repository
- `--since`: Fetch PRs created after a specific date (not implemented in Phase 1)
- `--until`: Fetch PRs created before a specific date (not implemented in Phase 1)
- `--incremental`: Resume from the last successful fetch (not implemented in Phase 1)

### Output

Data is exported in NDJSON (Newline Delimited JSON) format, with one pull request per line. This format is ideal for streaming processing and data analysis tools.

## Configuration

For GitHub Enterprise installations, create a configuration file at `~/.sirseer/config.yaml`:

```yaml
github:
  api_endpoint: https://github.your-company.com/api/v3
  graphql_endpoint: https://github.your-company.com/api/graphql
```

## License

This software is licensed under the Business Source License 1.1. See the [LICENSE](LICENSE) file for details.

## Support

For questions or issues, please contact: ip@sirseer.com