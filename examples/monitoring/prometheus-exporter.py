#!/usr/bin/env python3
"""
Prometheus exporter for SirSeer Relay metadata

Exposes metrics from sirseer-relay metadata.json files to Prometheus.
"""

import argparse
import json
import os
import time
from datetime import datetime
from pathlib import Path
from typing import Dict, List

from prometheus_client import start_http_server, Gauge, Counter, Histogram

# Define Prometheus metrics
fetch_duration = Histogram(
    'sirseer_fetch_duration_seconds',
    'Duration of fetch operation in seconds',
    ['repository', 'fetch_type']
)

prs_fetched = Gauge(
    'sirseer_prs_fetched_total',
    'Total number of PRs fetched',
    ['repository']
)

fetch_errors = Counter(
    'sirseer_fetch_errors_total',
    'Total number of fetch errors',
    ['repository', 'error_type']
)

last_fetch_timestamp = Gauge(
    'sirseer_last_fetch_timestamp',
    'Unix timestamp of last successful fetch',
    ['repository']
)

fetch_state = Gauge(
    'sirseer_fetch_state',
    'Current state of repository fetch (0=never_fetched, 1=success, 2=partial, 3=failed)',
    ['repository']
)

rate_limit_encounters = Counter(
    'sirseer_rate_limit_encounters_total',
    'Number of rate limit encounters',
    ['repository']
)

network_retries = Counter(
    'sirseer_network_retries_total',
    'Number of network retry attempts',
    ['repository']
)

output_file_size = Gauge(
    'sirseer_output_file_size_bytes',
    'Size of output NDJSON file in bytes',
    ['repository']
)


class MetadataCollector:
    """Collects and processes SirSeer metadata files"""
    
    def __init__(self, metadata_dir: str):
        self.metadata_dir = Path(metadata_dir)
        self.known_repos: Dict[str, dict] = {}
    
    def scan_metadata_files(self) -> List[Path]:
        """Find all metadata.json files in the directory"""
        return list(self.metadata_dir.glob("*_metadata.json"))
    
    def parse_metadata(self, filepath: Path) -> dict:
        """Parse a metadata file and extract metrics"""
        try:
            with open(filepath, 'r') as f:
                data = json.load(f)
            
            # Extract repository name from filename
            repo_name = filepath.stem.replace('_metadata', '')
            data['repository'] = repo_name.replace('_', '/')
            
            return data
        except Exception as e:
            print(f"Error parsing {filepath}: {e}")
            return None
    
    def update_metrics(self, metadata: dict):
        """Update Prometheus metrics from metadata"""
        repo = metadata['repository']
        
        # Fetch duration
        if 'duration' in metadata:
            fetch_type = 'full' if metadata.get('all', False) else 'incremental'
            fetch_duration.labels(
                repository=repo,
                fetch_type=fetch_type
            ).observe(metadata['duration'])
        
        # PRs fetched
        if 'pullRequestsFetched' in metadata:
            prs_fetched.labels(repository=repo).set(metadata['pullRequestsFetched'])
        
        # Last fetch timestamp
        if 'endTime' in metadata:
            # Parse ISO format timestamp
            try:
                dt = datetime.fromisoformat(metadata['endTime'].replace('Z', '+00:00'))
                timestamp = dt.timestamp()
                last_fetch_timestamp.labels(repository=repo).set(timestamp)
            except:
                pass
        
        # Fetch state
        if metadata.get('error'):
            fetch_state.labels(repository=repo).set(3)  # failed
            
            # Count errors by type
            error_type = 'unknown'
            error_msg = str(metadata.get('error', ''))
            
            if 'rate limit' in error_msg.lower():
                error_type = 'rate_limit'
                rate_limit_encounters.labels(repository=repo).inc()
            elif 'timeout' in error_msg.lower():
                error_type = 'timeout'
            elif 'network' in error_msg.lower():
                error_type = 'network'
            
            fetch_errors.labels(
                repository=repo,
                error_type=error_type
            ).inc()
        elif metadata.get('partial'):
            fetch_state.labels(repository=repo).set(2)  # partial
        else:
            fetch_state.labels(repository=repo).set(1)  # success
        
        # Network retries (if available in future metadata)
        if 'retryCount' in metadata:
            network_retries.labels(repository=repo).inc(metadata['retryCount'])
        
        # Output file size
        output_file = metadata.get('outputFile')
        if output_file:
            output_path = Path(output_file)
            if output_path.exists():
                size = output_path.stat().st_size
                output_file_size.labels(repository=repo).set(size)
    
    def collect_metrics(self):
        """Scan for metadata files and update metrics"""
        metadata_files = self.scan_metadata_files()
        
        for filepath in metadata_files:
            metadata = self.parse_metadata(filepath)
            if metadata:
                self.update_metrics(metadata)
                self.known_repos[metadata['repository']] = metadata
        
        # Set state for repos we haven't seen
        for repo in list(self.known_repos.keys()):
            if repo not in [m.get('repository') for m in 
                          [self.parse_metadata(f) for f in metadata_files] if m]:
                fetch_state.labels(repository=repo).set(0)  # never_fetched


def main():
    parser = argparse.ArgumentParser(
        description='Prometheus exporter for SirSeer Relay metrics'
    )
    parser.add_argument(
        '--port', type=int, default=9100,
        help='Port to expose metrics on (default: 9100)'
    )
    parser.add_argument(
        '--metadata-dir', type=str,
        default=os.environ.get('SIRSEER_METADATA_DIR', './metadata'),
        help='Directory containing metadata files (default: ./metadata)'
    )
    parser.add_argument(
        '--interval', type=int, default=30,
        help='Metric collection interval in seconds (default: 30)'
    )
    
    args = parser.parse_args()
    
    # Validate metadata directory
    metadata_dir = Path(args.metadata_dir)
    if not metadata_dir.exists():
        print(f"Error: Metadata directory does not exist: {metadata_dir}")
        return 1
    
    # Start Prometheus HTTP server
    start_http_server(args.port)
    print(f"Prometheus exporter started on port {args.port}")
    print(f"Monitoring metadata directory: {metadata_dir}")
    print(f"Collection interval: {args.interval}s")
    print(f"Metrics available at: http://localhost:{args.port}/metrics")
    
    # Create collector
    collector = MetadataCollector(str(metadata_dir))
    
    # Main loop
    try:
        while True:
            collector.collect_metrics()
            time.sleep(args.interval)
    except KeyboardInterrupt:
        print("\nShutting down exporter...")
        return 0


if __name__ == '__main__':
    exit(main())