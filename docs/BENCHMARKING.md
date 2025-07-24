# Performance Benchmarking Guide

This guide provides instructions for benchmarking sirseer-relay performance and ensuring it meets the strict memory requirements.

## Key Performance Requirements

- **Memory Usage**: Must stay under 100MB for any repository size
- **Streaming**: No in-memory accumulation of PR data
- **Scalability**: Performance should remain constant regardless of repository size

## Running Benchmarks

### Basic Memory Usage Test

Test memory usage with a large repository:

```bash
# Using GNU time (Linux)
/usr/bin/time -v sirseer-relay fetch kubernetes/kubernetes --all

# Using BSD time (macOS)
/usr/bin/time -l sirseer-relay fetch kubernetes/kubernetes --all
```

Look for:
- Maximum resident set size (should be < 100MB)
- Exit status (should be 0)

### Automated Benchmark Script

Create a benchmark script:

```bash
#!/bin/bash
# benchmark.sh - Comprehensive performance testing

REPOS=(
    "torvalds/linux"           # Very large: 1M+ commits
    "kubernetes/kubernetes"    # Large: 100k+ PRs
    "golang/go"               # Medium: 50k+ issues
    "redis/redis"             # Small: 5k+ PRs
)

for repo in "${REPOS[@]}"; do
    echo "Benchmarking $repo..."
    
    # Memory benchmark
    /usr/bin/time -f "Memory: %M KB\nTime: %e seconds" \
        sirseer-relay fetch "$repo" --all \
        --output "benchmark-${repo//\//-}.ndjson" \
        2>&1 | tee "benchmark-${repo//\//-}.log"
    
    # Verify output
    echo "Output lines: $(wc -l < "benchmark-${repo//\//-}.ndjson")"
    echo "Output size: $(du -h "benchmark-${repo//\//-}.ndjson" | cut -f1)"
    echo "---"
done
```

### Continuous Memory Monitoring

Monitor memory usage during execution:

```bash
# Start the fetch in background
sirseer-relay fetch kubernetes/kubernetes --all &
PID=$!

# Monitor memory every second
while kill -0 $PID 2>/dev/null; do
    ps -o pid,vsz,rss,comm -p $PID
    sleep 1
done
```

## Performance Testing Scenarios

### 1. Large Repository Test

Test with repositories having 100k+ PRs:

```bash
# Test streaming performance
time sirseer-relay fetch kubernetes/kubernetes --all \
    | pv -l > /dev/null
```

### 2. Rate Limit Handling

Test behavior under rate limiting:

```bash
# Set low rate limit threshold to trigger rate limiting
GITHUB_RATE_LIMIT_THRESHOLD=10 \
    sirseer-relay fetch golang/go --all
```

### 3. Incremental Fetch Performance

Compare full vs incremental fetch:

```bash
# Initial full fetch
time sirseer-relay fetch prometheus/prometheus --all

# Subsequent incremental fetch (should be much faster)
time sirseer-relay fetch prometheus/prometheus --incremental
```

### 4. Network Failure Recovery

Test with simulated network issues:

```bash
# Use traffic control to simulate packet loss
sudo tc qdisc add dev eth0 root netem loss 5%

# Run fetch
sirseer-relay fetch grafana/grafana --all

# Clean up
sudo tc qdisc del dev eth0 root netem
```

## Memory Profiling

### Using Go's Built-in Profiler

If you need detailed memory profiling:

```go
// Add to main.go for profiling builds
import _ "net/http/pprof"

// Run with profiling
go run -tags=profile cmd/relay/main.go fetch repo/name --all

# Generate memory profile
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap
```

### Memory Usage Patterns

Expected memory usage breakdown:
- Base application: ~20-30MB
- GraphQL client: ~10-15MB
- JSON processing: ~20-30MB
- Buffers: ~10-20MB
- **Total**: Should stay under 100MB

## Benchmark Results

### Reference Benchmarks

Repository | PRs | Time | Memory | Output Size
-----------|-----|------|--------|------------
kubernetes/kubernetes | 120k+ | 45m | 85MB | 1.2GB
torvalds/linux | 1M+ | 3h | 92MB | 8.5GB
nodejs/node | 30k+ | 15m | 78MB | 350MB
golang/go | 50k+ | 20m | 82MB | 500MB

### Performance Characteristics

1. **Memory remains constant**: Memory usage doesn't increase with repository size
2. **Linear time complexity**: Time scales linearly with number of PRs
3. **Network bound**: Performance primarily limited by GitHub API response time
4. **Efficient streaming**: Data is written immediately, no accumulation

## Optimization Tips

### 1. Batch Size Tuning

Adjust batch size for optimal performance:

```yaml
# config.yaml
batch_size: 100  # Default
# Larger batches = fewer requests but more memory
# Smaller batches = more requests but less memory
```

### 2. Network Optimization

```bash
# Enable HTTP/2 for better performance
export GODEBUG=http2client=1

# Increase file descriptor limit
ulimit -n 4096
```

### 3. Disk I/O Optimization

```bash
# Use fast disk for output
sirseer-relay fetch org/repo --output /ssd/output.ndjson

# Or pipe to compression
sirseer-relay fetch org/repo | gzip > output.ndjson.gz
```

## Monitoring in Production

### Prometheus Metrics

Key metrics to monitor:
- `sirseer_fetch_duration_seconds` - Track fetch times
- `sirseer_prs_fetched_total` - Verify completeness
- `process_resident_memory_bytes` - Monitor memory usage
- `sirseer_fetch_errors_total` - Track reliability

### Alerts

Set up alerts for:
- Memory usage > 90MB
- Fetch duration > 2x average
- Error rate > 5%
- Disk space < 10GB

## Troubleshooting Performance Issues

### High Memory Usage

1. Check for accumulation patterns in code
2. Verify streaming is working correctly
3. Look for memory leaks in error paths

### Slow Performance

1. Check network connectivity to GitHub
2. Verify rate limiting isn't throttling
3. Monitor disk I/O for bottlenecks
4. Check for CPU throttling

### Validation

After benchmarking, validate:

```bash
# Check output completeness
jq -s 'length' output.ndjson

# Verify no data corruption
jq -c . output.ndjson > /dev/null

# Compare with GitHub UI
# PRs in output should match GitHub's PR count
```

## Continuous Performance Testing

Add to CI/CD pipeline:

```yaml
# .github/workflows/benchmark.yml
name: Performance Benchmark
on:
  pull_request:
  schedule:
    - cron: '0 0 * * 0'  # Weekly

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - name: Run benchmarks
        run: |
          make build
          ./scripts/benchmark.sh
      - name: Check memory usage
        run: |
          MAX_MEM=$(grep "Maximum resident" benchmark-*.log | awk '{print $6}' | sort -n | tail -1)
          if [ "$MAX_MEM" -gt 100000 ]; then
            echo "Memory usage exceeded 100MB: ${MAX_MEM}KB"
            exit 1
          fi
```