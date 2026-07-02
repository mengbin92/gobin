#!/bin/bash
# Validate benchmark output and fail on regressions.
#
# Two layers of checks:
#  1. Absolute ceiling: ns/op must be below an order-of-magnitude threshold
#     that catches "100x slower" regressions across heterogeneous CI hardware.
#  2. Relative regression: if a previous baseline (HEAD:benchmark-results.txt)
#     exists, current ns/op > 1.5x baseline -> red, > 1.2x -> yellow.
#     Skipped when benchmark noise (multiple runs) is too high.
#
# Thresholds can be overridden via env vars:
#   GOBIN_BENCH_FAIL_RATIO=1.5   GOBIN_BENCH_WARN_RATIO=1.2
#   GOBIN_BENCH_NOISE_RATIO=0.30

set -euo pipefail

RESULT_FILE=${1:-benchmark-results.txt}
FAIL_RATIO=${GOBIN_BENCH_FAIL_RATIO:-1.5}
WARN_RATIO=${GOBIN_BENCH_WARN_RATIO:-1.2}
NOISE_RATIO=${GOBIN_BENCH_NOISE_RATIO:-0.30}

if [ ! -f "$RESULT_FILE" ]; then
    echo "[error] benchmark result file not found: ${RESULT_FILE}"
    exit 1
fi

# Names of benchmarks that participate in regression checks.
BENCHMARKS=(
    "BenchmarkLoadConfig"
    "BenchmarkPaginate"
    "BenchmarkCopyFile"
    "BenchmarkParsePost"
    "BenchmarkParsePosts"
)

# Absolute ceilings (ns/op). These are intentionally broad to catch
# order-of-magnitude regressions, not normal benchmark noise.
declare -A CEILINGS=(
    ["BenchmarkLoadConfig"]=10000000
    ["BenchmarkPaginate"]=1000000
    ["BenchmarkCopyFile"]=20000000
    ["BenchmarkParsePost"]=20000000
    ["BenchmarkParsePosts"]=50000000
)

# Get the ns/op of the first matching benchmark line. The format is:
#   BenchmarkName[-N]  <iter>  <ns/op>  <B/op>  <allocs/op>
ns_of() {
    local name=$1
    local file=$2
    grep -E "^${name}-[0-9]+" "$file" | head -n 1 | awk '{print $3}' || true
}

# Compute the geometric mean of multiple ns/op values (one per -count=N run).
# Returns empty if the line is missing. Used to assess benchmark noise.
noise_ratio() {
    local name=$1
    local file=$2
    local values
    values=$(grep -E "^${name}-[0-9]+" "$file" | awk '{print $3}' || true)
    if [ -z "$values" ]; then
        return 1
    fi
    # mean / max-min difference ratio: lower = less noise. We use a simple
    # coefficient of variation proxy: (max-min) / mean. If the proxy exceeds
    # NOISE_RATIO, the relative check is skipped for this benchmark.
    local stats
    stats=$(echo "$values" | awk '
        BEGIN { min=999999999; max=0; sum=0; n=0 }
        { if ($1 < min) min = $1; if ($1 > max) max = $1; sum += $1; n++ }
        END {
            if (n == 0) exit 1
            mean = sum / n
            if (mean == 0) exit 1
            print (max - min) / mean
        }
    ')
    echo "$stats"
}

# 1. Absolute ceiling checks.
echo "[info] running absolute ceiling checks"
for name in "${BENCHMARKS[@]}"; do
    ceiling=${CEILINGS[$name]}
    ns=$(ns_of "$name" "$RESULT_FILE")
    if [ -z "$ns" ]; then
        echo "[error] missing benchmark result for ${name}"
        exit 1
    fi
    if [ "$ns" -gt "$ceiling" ]; then
        echo "[error] ${name} exceeded ceiling: ${ns} ns/op > ${ceiling} ns/op"
        exit 1
    fi
    echo "[ok]   ${name}: ${ns} ns/op <= ${ceiling} ns/op"
done

# 2. Relative regression checks against HEAD:benchmark-results.txt.
if git rev-parse --git-dir >/dev/null 2>&1 && git show "HEAD:${RESULT_FILE}" >/dev/null 2>&1; then
    BASELINE=$(mktemp)
    trap "rm -f $BASELINE" EXIT
    git show "HEAD:${RESULT_FILE}" > "$BASELINE"
    echo ""
    echo "[info] running relative regression checks vs HEAD baseline"
    any_warn=0
    any_fail=0
    for name in "${BENCHMARKS[@]}"; do
        current=$(ns_of "$name" "$RESULT_FILE")
        previous=$(ns_of "$name" "$BASELINE")
        if [ -z "$previous" ] || [ -z "$current" ] || [ "$previous" -eq 0 ]; then
            continue
        fi

        # Noise gate: if the current run is too noisy, skip the relative check.
        noise=$(noise_ratio "$name" "$RESULT_FILE" || echo "")
        if [ -n "$noise" ]; then
            # shellcheck disable=SC2086
            if awk "BEGIN { exit !($noise > $NOISE_RATIO) }"; then
                echo "[skip] ${name}: noise ratio ${noise} > ${NOISE_RATIO}; skipping relative check"
                continue
            fi
        fi

        # Use awk to avoid bash integer-only arithmetic.
        ratio=$(awk -v c="$current" -v p="$previous" 'BEGIN { if (p == 0) { print 0 } else { printf "%.4f", c / p } }')
        if awk -v r="$ratio" -v f="$FAIL_RATIO" 'BEGIN { exit !(r > f) }'; then
            echo "[fail] ${name}: ${current} ns/op > ${FAIL_RATIO}x baseline (${previous} ns/op, ratio=${ratio})"
            any_fail=1
        elif awk -v r="$ratio" -v w="$WARN_RATIO" 'BEGIN { exit !(r > w) }'; then
            echo "[warn] ${name}: ${current} ns/op > ${WARN_RATIO}x baseline (${previous} ns/op, ratio=${ratio})"
            any_warn=1
        else
            echo "[ok]   ${name}: ${current} ns/op (ratio=${ratio} <= ${WARN_RATIO})"
        fi
    done
    if [ "$any_fail" -ne 0 ]; then
        exit 1
    fi
    if [ "$any_warn" -ne 0 ]; then
        # CI default is fail-on-warn; local default is warn-only. Honor
        # GOBIN_BENCH_FAIL_ON_WARN for CI.
        if [ "${GOBIN_BENCH_FAIL_ON_WARN:-1}" = "1" ]; then
            echo "[error] relative regression check produced warnings; failing because GOBIN_BENCH_FAIL_ON_WARN=1"
            exit 1
        fi
        echo "[warn] relative regression check produced warnings; set GOBIN_BENCH_FAIL_ON_WARN=1 to fail"
    fi
else
    echo "[skip] no HEAD baseline found; skipping relative checks"
fi

echo ""
echo "[ok] benchmark checks passed"
