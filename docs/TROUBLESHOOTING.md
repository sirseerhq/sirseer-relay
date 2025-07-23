# Troubleshooting Guide

This guide helps you resolve common issues with sirseer-relay.

## Table of Contents

1. [Authentication Issues](#authentication-issues)
2. [Network and Connectivity](#network-and-connectivity)
3. [Performance Issues](#performance-issues)
4. [State Management Issues](#state-management-issues)
5. [Output and Data Issues](#output-and-data-issues)
6. [GitHub API Limits](#github-api-limits)
7. [Enterprise GitHub](#enterprise-github)
8. [Getting Help](#getting-help)

## Authentication Issues

### Error: "GitHub token not found"

**Symptom:**
```
Error: GitHub token not found. Set GITHUB_TOKEN or use --token flag
```

**Solutions:**

1. **Set environment variable:**
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   sirseer-relay fetch owner/repo
   ```

2. **Use --token flag:**
   ```bash
   sirseer-relay fetch owner/repo --token ghp_your_token_here
   ```

3. **Add to shell profile (permanent):**
   ```bash
   echo 'export GITHUB_TOKEN=ghp_your_token_here' >> ~/.bashrc
   source ~/.bashrc
   ```

### Error: "401 Unauthorized" or "403 Forbidden"

**Symptom:**
```
Error: GitHub API authentication failed. Please verify your GITHUB_TOKEN.
```

**Causes and Solutions:**

1. **Invalid token:**
   - Verify token at: https://github.com/settings/tokens
   - Regenerate if needed

2. **Insufficient permissions:**
   - Token needs `repo` scope for private repositories
   - Token needs `public_repo` scope for public repositories

3. **Token expired:**
   - GitHub tokens can expire
   - Create a new token with appropriate expiration

4. **SSO enforcement:**
   ```bash
   # For organizations with SSO
   # Authorize your token at:
   https://github.com/settings/tokens
   # Click "Configure SSO" next to your token
   ```

## Network and Connectivity

### Error: "Network error connecting to GitHub API"

**Symptom:**
```
Error: network error connecting to GitHub API. Please check your internet connection and try again.
```

**Solutions:**

1. **Check basic connectivity:**
   ```bash
   # Test GitHub API
   curl -H "Authorization: Bearer $GITHUB_TOKEN" https://api.github.com/user
   ```

2. **Behind a proxy:**
   ```bash
   export HTTP_PROXY=http://proxy.company.com:8080
   export HTTPS_PROXY=http://proxy.company.com:8080
   ```

3. **DNS issues:**
   ```bash
   # Test DNS resolution
   nslookup api.github.com
   
   # Try alternative DNS
   echo "nameserver 8.8.8.8" | sudo tee /etc/resolv.conf
   ```

4. **Firewall blocking:**
   - Ensure port 443 (HTTPS) is open
   - Check with your IT department

### Timeout Errors

**Symptom:**
```
Error: Request timeout after 180 seconds
```

**Solutions:**

1. **Increase timeout:**
   ```bash
   sirseer-relay fetch owner/repo --all --request-timeout 600
   ```

2. **Use incremental fetching:**
   ```bash
   # Fetch in smaller chunks
   sirseer-relay fetch owner/repo --since 2024-01-01 --until 2024-06-30
   sirseer-relay fetch owner/repo --since 2024-07-01 --until 2024-12-31
   ```

## Performance Issues

### Slow Fetching Speed

**Possible Causes:**

1. **Large repository:**
   - Some repos have 100k+ PRs
   - Use progress indicator to monitor

2. **API rate limits:**
   - Check remaining quota (see [GitHub API Limits](#github-api-limits))

3. **Network latency:**
   ```bash
   # Test latency to GitHub
   ping api.github.com
   ```

**Solutions:**

1. **Use time windows:**
   ```bash
   # Fetch year by year
   sirseer-relay fetch owner/repo --since 2023-01-01 --until 2023-12-31
   ```

2. **Optimize for your use case:**
   ```bash
   # If you only need recent PRs
   sirseer-relay fetch owner/repo --since 30d
   ```

### High Memory Usage

**Note:** sirseer-relay is designed to use < 100MB of memory regardless of repository size.

If experiencing high memory:

1. **Check version:**
   ```bash
   sirseer-relay version
   ```

2. **Monitor actual usage:**
   ```bash
   # Linux/Mac
   /usr/bin/time -v sirseer-relay fetch owner/repo --all
   
   # Look for "Maximum resident set size"
   ```

3. **Report issue:**
   - This would be a bug
   - Report at: https://github.com/sirseerhq/sirseer-relay/issues

## State Management Issues

### Error: "State file is corrupted"

See detailed recovery in [State Management Guide](STATE_MANAGEMENT.md#corrupted-state-file).

Quick fix:
```bash
rm ~/.sirseer/state/owner-repo.state
sirseer-relay fetch owner/repo --all
```

### Error: "No previous fetch state found"

**For incremental fetch to work:**
```bash
# First, run a full fetch
sirseer-relay fetch owner/repo --all

# Then incremental works
sirseer-relay fetch owner/repo --incremental
```

### State File Permissions

**Symptom:**
```
Error: Permission denied writing state file
```

**Solution:**
```bash
# Fix permissions
chmod 755 ~/.sirseer
chmod 755 ~/.sirseer/state
chmod 644 ~/.sirseer/state/*.state
```

## Output and Data Issues

### Empty Output

**Possible Causes:**

1. **No PRs in repository:**
   ```bash
   # Verify repository has PRs
   curl -H "Authorization: Bearer $GITHUB_TOKEN" \
        https://api.github.com/repos/owner/repo/pulls?state=all
   ```

2. **Date filters too restrictive:**
   ```bash
   # Check your date range
   sirseer-relay fetch owner/repo --since 2024-01-01 --until 2024-01-01
   # Only returns PRs created on exactly that date
   ```

3. **Writing to file:**
   ```bash
   # Check if file was created
   ls -la output.ndjson
   ```

### Malformed JSON Output

**Symptom:**
```
Error parsing JSON: unexpected end of JSON input
```

**Solutions:**

1. **Check for partial writes:**
   ```bash
   # Validate NDJSON format
   cat output.ndjson | while read line; do echo $line | jq . > /dev/null || echo "Invalid: $line"; done
   ```

2. **Ensure clean exit:**
   - Don't interrupt with Ctrl+C during write
   - Check exit code: `echo $?`

3. **Remove incomplete lines:**
   ```bash
   # Remove last line if incomplete
   sed -i '$ d' output.ndjson
   ```

## GitHub API Limits

### Rate Limit Exceeded

**Symptom:**
```
Error: GitHub API rate limit exceeded. Please wait before retrying.
```

**Check your limits:**
```bash
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
     https://api.github.com/rate_limit
```

**Solutions:**

1. **Wait for reset:**
   - Check `X-RateLimit-Reset` header
   - Usually resets within an hour

2. **Use incremental fetching:**
   - Reduces API calls needed

3. **Upgrade token limits:**
   - GitHub Apps have higher limits
   - Enterprise accounts have higher limits

### Query Complexity Errors

**Symptom:**
```
Query complexity limit hit. Reducing page size to 25...
```

**This is handled automatically!** The tool will:
1. Detect complexity errors
2. Reduce batch size
3. Continue fetching
4. Show notification in progress

## Enterprise GitHub

### Configuration Issues

**For GitHub Enterprise:**

1. **Create config file:**
   ```bash
   mkdir -p ~/.sirseer
   cat > ~/.sirseer/config.yaml << EOF
   github:
     api_endpoint: https://github.company.com/api/v3
     graphql_endpoint: https://github.company.com/api/graphql
   EOF
   ```

2. **Verify endpoints:**
   ```bash
   # Test API endpoint
   curl https://github.company.com/api/v3
   
   # Test GraphQL endpoint
   curl -X POST https://github.company.com/api/graphql
   ```

3. **SSL Certificate Issues:**
   ```bash
   # For self-signed certificates (not recommended for production)
   export NODE_TLS_REJECT_UNAUTHORIZED=0
   ```

### SSO and 2FA

**For organizations with SSO:**

1. Authorize your token for SSO
2. Visit: https://github.com/settings/tokens
3. Click "Configure SSO" next to token
4. Authorize for required organizations

## Getting Help

### Debug Mode

Run with verbose output:
```bash
# Not yet implemented, but planned
sirseer-relay fetch owner/repo --debug
```

### Collecting Diagnostic Information

When reporting issues, include:

1. **Version:**
   ```bash
   sirseer-relay version
   ```

2. **Error message:**
   ```bash
   sirseer-relay fetch owner/repo 2>&1 | tee error.log
   ```

3. **Environment:**
   ```bash
   echo "OS: $(uname -a)"
   echo "Go version: $(go version)"
   echo "Token scopes: $(curl -sH "Authorization: Bearer $GITHUB_TOKEN" https://api.github.com/user | jq -r '.scopes')"
   ```

4. **Repository info:**
   - Repository name (if public)
   - Approximate size (number of PRs)
   - Enterprise or GitHub.com

### Support Channels

1. **GitHub Issues:**
   - https://github.com/sirseerhq/sirseer-relay/issues

2. **Email Support:**
   - ip@sirseer.com

3. **Documentation:**
   - [Usage Guide](USAGE.md)
   - [State Management](STATE_MANAGEMENT.md)
   - [Examples](../examples/)

### Common Quick Fixes

```bash
# Reset everything and start fresh
rm -rf ~/.sirseer/state/
rm output.ndjson
export GITHUB_TOKEN=ghp_your_token
sirseer-relay fetch owner/repo --all --output fresh-output.ndjson

# Test with a small public repo
sirseer-relay fetch octocat/Hello-World --all
```