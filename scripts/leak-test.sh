#!/bin/bash
#
# ZimaOS-Blue Leak Detection Script
# Detects memory and goroutine leaks using pprof
#

set -e

# Configuration
SERVICE_URL="${SERVICE_URL:-http://localhost:23456}"
PPROF_URL="${SERVICE_URL}/debug/pprof"
OUTPUT_DIR="${OUTPUT_DIR:-./leak-test-results}"
SAMPLE_INTERVAL="${SAMPLE_INTERVAL:-60}"  # seconds
SAMPLE_COUNT="${SAMPLE_COUNT:-10}"
LOAD_DURATION="${LOAD_DURATION:-300}"  # 5 minutes of load

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }

check_dependencies() {
    if ! command -v go &> /dev/null; then
        log_error "Go is required for pprof analysis"
        exit 1
    fi

    if ! command -v curl &> /dev/null; then
        log_error "curl is required"
        exit 1
    fi
}

setup() {
    mkdir -p "$OUTPUT_DIR"/{heap,goroutine,allocs}
    log_info "Output directory: $OUTPUT_DIR"
}

check_pprof_available() {
    if ! curl -sf --max-time 5 "$PPROF_URL/" > /dev/null 2>&1; then
        log_error "pprof endpoints not available at $PPROF_URL"
        log_info "Make sure profiling is enabled in config"
        exit 1
    fi
    log_success "pprof endpoints available"
}

collect_baseline() {
    log_info "Collecting baseline profiles..."

    # Heap profile
    curl -sf "$PPROF_URL/heap" > "$OUTPUT_DIR/heap/baseline.pb.gz"

    # Goroutine profile
    curl -sf "$PPROF_URL/goroutine" > "$OUTPUT_DIR/goroutine/baseline.pb.gz"

    # Allocs profile
    curl -sf "$PPROF_URL/allocs" > "$OUTPUT_DIR/allocs/baseline.pb.gz"

    log_success "Baseline profiles collected"
}

generate_load() {
    log_info "Generating load for ${LOAD_DURATION}s..."

    local end_time=$(($(date +%s) + LOAD_DURATION))
    local requests=0

    while [ $(date +%s) -lt $end_time ]; do
        # Send various requests
        curl -sf "$SERVICE_URL/health" > /dev/null 2>&1 &
        curl -sf "$SERVICE_URL/api/v1/workers/stats" > /dev/null 2>&1 &

        requests=$((requests + 2))

        # Don't overwhelm
        sleep 0.1
    done

    wait
    log_info "Generated $requests requests"
}

collect_samples() {
    log_info "Collecting $SAMPLE_COUNT samples at ${SAMPLE_INTERVAL}s intervals..."

    for i in $(seq 1 $SAMPLE_COUNT); do
        log_info "Sample $i/$SAMPLE_COUNT"

        # Heap profile
        curl -sf "$PPROF_URL/heap" > "$OUTPUT_DIR/heap/sample_$i.pb.gz"

        # Goroutine profile
        curl -sf "$PPROF_URL/goroutine" > "$OUTPUT_DIR/goroutine/sample_$i.pb.gz"

        # Allocs profile
        curl -sf "$PPROF_URL/allocs" > "$OUTPUT_DIR/allocs/sample_$i.pb.gz"

        if [ $i -lt $SAMPLE_COUNT ]; then
            sleep "$SAMPLE_INTERVAL"
        fi
    done

    log_success "All samples collected"
}

analyze_heap() {
    log_info "Analyzing heap profiles..."

    local baseline_size=$(stat -f%z "$OUTPUT_DIR/heap/baseline.pb.gz" 2>/dev/null || stat -c%s "$OUTPUT_DIR/heap/baseline.pb.gz")
    local final_size=$(stat -f%z "$OUTPUT_DIR/heap/sample_$SAMPLE_COUNT.pb.gz" 2>/dev/null || stat -c%s "$OUTPUT_DIR/heap/sample_$SAMPLE_COUNT.pb.gz")

    # Compare baseline to final
    go tool pprof -text -base "$OUTPUT_DIR/heap/baseline.pb.gz" "$OUTPUT_DIR/heap/sample_$SAMPLE_COUNT.pb.gz" > "$OUTPUT_DIR/heap_diff.txt" 2>/dev/null || true

    # Check for significant growth
    local growth_lines=$(grep -c "^[[:space:]]*[0-9]" "$OUTPUT_DIR/heap_diff.txt" 2>/dev/null || echo "0")

    if [ "$growth_lines" -gt 20 ]; then
        log_warn "Potential memory leak detected - $growth_lines allocation sites growing"
        return 1
    else
        log_success "No significant heap growth detected"
        return 0
    fi
}

analyze_goroutines() {
    log_info "Analyzing goroutine profiles..."

    # Get goroutine counts
    local baseline_count=$(curl -sf "$PPROF_URL/goroutine?debug=1" 2>/dev/null | head -1 | grep -o '[0-9]*' || echo "0")

    # Wait and check again
    sleep 5

    local final_count=$(curl -sf "$PPROF_URL/goroutine?debug=1" 2>/dev/null | head -1 | grep -o '[0-9]*' || echo "0")

    echo "Baseline goroutines: $baseline_count" > "$OUTPUT_DIR/goroutine_analysis.txt"
    echo "Final goroutines: $final_count" >> "$OUTPUT_DIR/goroutine_analysis.txt"

    if [ -n "$baseline_count" ] && [ -n "$final_count" ] && [ "$baseline_count" -gt 0 ]; then
        local growth=$(( (final_count - baseline_count) * 100 / baseline_count ))
        echo "Growth: ${growth}%" >> "$OUTPUT_DIR/goroutine_analysis.txt"

        if [ "$growth" -gt 50 ]; then
            log_warn "Potential goroutine leak: ${growth}% growth ($baseline_count -> $final_count)"

            # Get goroutine dump for analysis
            curl -sf "$PPROF_URL/goroutine?debug=2" > "$OUTPUT_DIR/goroutine_dump.txt" 2>/dev/null || true

            return 1
        else
            log_success "Goroutine count stable (${growth}% change)"
            return 0
        fi
    else
        log_warn "Could not determine goroutine counts"
        return 0
    fi
}

analyze_allocs() {
    log_info "Analyzing allocation profiles..."

    # Compare baseline to final
    go tool pprof -text -base "$OUTPUT_DIR/allocs/baseline.pb.gz" "$OUTPUT_DIR/allocs/sample_$SAMPLE_COUNT.pb.gz" > "$OUTPUT_DIR/allocs_diff.txt" 2>/dev/null || true

    # Look for high allocation rates
    local high_alloc=$(grep -E "^[[:space:]]*[0-9]+\.[0-9]+[MG]B" "$OUTPUT_DIR/allocs_diff.txt" 2>/dev/null | wc -l || echo "0")

    if [ "$high_alloc" -gt 5 ]; then
        log_warn "High allocation rate detected in $high_alloc sites"
        return 1
    else
        log_success "Allocation rates normal"
        return 0
    fi
}

generate_report() {
    local report="$OUTPUT_DIR/leak_report.txt"

    cat > "$report" << EOF
ZimaOS-Blue Leak Detection Report
==================================
Generated: $(date '+%Y-%m-%d %H:%M:%S')

Test Configuration:
  Service URL: $SERVICE_URL
  Sample Count: $SAMPLE_COUNT
  Sample Interval: ${SAMPLE_INTERVAL}s
  Load Duration: ${LOAD_DURATION}s

Results:
EOF

    if [ -f "$OUTPUT_DIR/heap_diff.txt" ]; then
        echo "" >> "$report"
        echo "Heap Analysis:" >> "$report"
        head -20 "$OUTPUT_DIR/heap_diff.txt" >> "$report"
    fi

    if [ -f "$OUTPUT_DIR/goroutine_analysis.txt" ]; then
        echo "" >> "$report"
        echo "Goroutine Analysis:" >> "$report"
        cat "$OUTPUT_DIR/goroutine_analysis.txt" >> "$report"
    fi

    if [ -f "$OUTPUT_DIR/allocs_diff.txt" ]; then
        echo "" >> "$report"
        echo "Allocation Analysis:" >> "$report"
        head -20 "$OUTPUT_DIR/allocs_diff.txt" >> "$report"
    fi

    log_info "Report generated: $report"
}

main() {
    echo ""
    echo "=============================================="
    echo "  ZimaOS-Blue Leak Detection"
    echo "=============================================="
    echo ""

    check_dependencies
    setup
    check_pprof_available

    collect_baseline

    # Generate load in background
    generate_load &
    local load_pid=$!

    # Collect samples while load is running
    collect_samples

    # Wait for load to finish
    wait $load_pid 2>/dev/null || true

    # Analyze results
    local heap_ok=true
    local goroutine_ok=true
    local allocs_ok=true

    analyze_heap || heap_ok=false
    analyze_goroutines || goroutine_ok=false
    analyze_allocs || allocs_ok=false

    generate_report

    echo ""
    echo "=============================================="
    echo "  Summary"
    echo "=============================================="

    if $heap_ok && $goroutine_ok && $allocs_ok; then
        log_success "No leaks detected!"
        exit 0
    else
        log_error "Potential leaks detected - review $OUTPUT_DIR for details"
        exit 1
    fi
}

main "$@"
