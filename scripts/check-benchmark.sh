#!/bin/bash
# Validate benchmark output and fail on obvious performance regressions.

set -euo pipefail

RESULT_FILE=${1:-benchmark-results.txt}

if [ ! -f "$RESULT_FILE" ]; then
    echo "[error] benchmark result file not found: ${RESULT_FILE}"
    exit 1
fi

check_benchmark() {
    local name=$1
    local max_ns=$2
    local line
    local ns

    line=$(grep -E "^${name}-[0-9]+" "$RESULT_FILE" | head -n 1 || true)
    if [ -z "$line" ]; then
        echo "[error] missing benchmark result for ${name}"
        exit 1
    fi

    ns=$(echo "$line" | awk '{print $3}')
    if ! [[ "$ns" =~ ^[0-9]+$ ]]; then
        echo "[error] could not parse ns/op for ${name}: ${line}"
        exit 1
    fi

    if [ "$ns" -gt "$max_ns" ]; then
        echo "[error] ${name} exceeded threshold: ${ns} ns/op > ${max_ns} ns/op"
        exit 1
    fi

    echo "[ok] ${name}: ${ns} ns/op <= ${max_ns} ns/op"
}

# Thresholds are intentionally broad to catch order-of-magnitude regressions
# across different CI hardware, not normal benchmark noise.
check_benchmark "BenchmarkLoadConfig" 10000000
check_benchmark "BenchmarkPaginate" 1000000
check_benchmark "BenchmarkCopyFile" 20000000
check_benchmark "BenchmarkParsePost" 20000000
check_benchmark "BenchmarkParsePosts" 50000000

echo "[ok] benchmark thresholds passed"
