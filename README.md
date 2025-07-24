# SirSeer Relay

A high-performance tool for extracting pull request metadata from GitHub repositories.

## Quick Start

```bash
# Set your GitHub token
export GITHUB_TOKEN=ghp_your_token_here

# Fetch recent PRs
sirseer-relay fetch golang/go

# Fetch all PRs from a repository
sirseer-relay fetch kubernetes/kubernetes --all
```

## Features

- 📊 **Stream Large Repositories** - Handles 100k+ PRs with < 100MB memory
- 🔄 **Incremental Updates** - Fetch only new PRs since last run
- 📅 **Time Window Filtering** - Extract PRs from specific date ranges
- 🚀 **Fast & Reliable** - Automatic retry and progress tracking
- 🏢 **Enterprise Ready** - Supports GitHub Enterprise Server

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

## Authentication

### Creating a GitHub Token

You can create a new Personal Access Token with the required permissions at:
[**github.com/settings/tokens**](https://github.com/settings/tokens)

For public repositories, you only need the `public_repo` scope.

### Token Management Best Practices

For development and testing:

1. **Environment File (Not Auto-loaded)**
   ```bash
   # Create a .env file (gitignored)
   echo 'GITHUB_TOKEN=ghp_your_token_here' > .env
   # Load manually when needed
   source .env && sirseer-relay fetch owner/repo
   ```

2. **Shell Profile (Persistent)**
   ```bash
   # Add to ~/.bashrc or ~/.zshrc
   export GITHUB_TOKEN=ghp_your_token_here
   ```

3. **Secure Token Storage**
   - Never commit tokens to version control
   - Use minimal token permissions (public_repo for public repos)
   - Rotate tokens regularly
   - Consider using GitHub App tokens for production

## Common Usage Patterns

### Fetch All PRs
```bash
sirseer-relay fetch owner/repo --all --output prs.ndjson
```

### Incremental Updates
```bash
# First run: full fetch
sirseer-relay fetch owner/repo --all

# Daily updates: only new PRs
sirseer-relay fetch owner/repo --incremental
```

### Time Window Filtering
```bash
# Fetch PRs from Q1 2024
sirseer-relay fetch owner/repo --since 2024-01-01 --until 2024-03-31
```

### Key Options

- `--all` - Fetch complete PR history
- `--incremental` - Fetch only new PRs since last run
- `--since` / `--until` - Filter by creation date
- `--output` - Save to file (default: stdout)
- `--token` - Override GITHUB_TOKEN env var

## Documentation

For detailed guides and advanced usage:

- 📖 **[Usage Guide](docs/USAGE.md)** - Comprehensive examples and patterns
- 🔧 **[State Management](docs/STATE_MANAGEMENT.md)** - How incremental fetching works
- ❓ **[Troubleshooting](docs/TROUBLESHOOTING.md)** - Common issues and solutions
- 📝 **[Examples](examples/)** - Automation scripts and workflows

## Output Format

Data is exported in **NDJSON** (Newline Delimited JSON) format:

```json
{"number":1234,"title":"Add feature","state":"closed","created_at":"2024-01-15T10:30:00Z",...}
{"number":1235,"title":"Fix bug","state":"open","created_at":"2024-01-16T14:20:00Z",...}
```

Perfect for streaming processing with tools like `jq`, `awk`, or data pipelines.

## Enterprise GitHub

For GitHub Enterprise Server, create `~/.sirseer/config.yaml`:

```yaml
github:
  api_endpoint: https://github.company.com/api/v3
  graphql_endpoint: https://github.company.com/api/graphql
```

Then use sirseer-relay normally. See [Troubleshooting](docs/TROUBLESHOOTING.md#enterprise-github) for more details.

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