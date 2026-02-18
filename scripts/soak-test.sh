#!/bin/bash
#
# ZimaOS-Blue Soak Test Script
# Long-running stability test
#

set -e

# Configuration
SERVICE_NAME="${SERVICE_NAME:-zimaos-blue}"
HEALTH_URL="${HEALTH_URL:-http://localhost/health}"
METRICS_URL="${METRICS_URL:-http://localhost/metrics}"
DURATION_HOURS="${DURATION_HOURS:-168}"  # 7 days default
CHECK_INTERVAL="${CHECK_INTERVAL:-60}"   # seconds
REPORT_INTERVAL="${REPORT_INTERVAL:-3600}"  # hourly reports
OUTPUT_DIR="${OUTPUT_DIR:-./soak-test-results}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Metrics
START_TIME=$(date +%s)
TOTAL_CHECKS=0
FAILED_CHECKS=0
MAX_MEMORY=0
MAX_GOROUTINES=0
ERRORS=()

log_info() { echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"; }
log_success() { echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"; }
log_error() { echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"; }

setup() {
    mkdir -p "$OUTPUT_DIR"

    # Create CSV header
    echo "timestamp,status,memory_mb,goroutines,uptime,response_time_ms" > "$OUTPUT_DIR/metrics.csv"

    log_info "Soak test started"
    log_info "Duration: ${DURATION_HOURS} hours"
    log_info "Check interval: ${CHECK_INTERVAL}s"
    log_info "Output directory: $OUTPUT_DIR"
}

check_health() {
    local start_ms=$(date +%s%3N)
    local response
    local status="ok"

    response=$(curl -sf --max-time 10 "$HEALTH_URL" 2>/dev/null) || status="failed"

    local end_ms=$(date +%s%3N)
    local response_time=$((end_ms - start_ms))

    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

    if [ "$status" = "failed" ]; then
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
        ERRORS+=("$(date '+%Y-%m-%d %H:%M:%S'): Health check failed")
        return 1
    fi

    # Parse response
    local memory=$(echo "$response" | grep -o '"mem_alloc_bytes":[0-9]*' | grep -o '[0-9]*' || echo "0")
    local goroutines=$(echo "$response" | grep -o '"goroutines":[0-9]*' | grep -o '[0-9]*' || echo "0")
    local uptime=$(echo "$response" | grep -o '"uptime":"[^"]*"' | cut -d'"' -f4 || echo "unknown")

    # Convert memory to MB
    local memory_mb=$((memory / 1024 / 1024))

    # Track maximums
    if [ "$memory_mb" -gt "$MAX_MEMORY" ]; then
        MAX_MEMORY=$memory_mb
    fi
    if [ "$goroutines" -gt "$MAX_GOROUTINES" ]; then
        MAX_GOROUTINES=$goroutines
    fi

    # Log to CSV
    echo "$(date '+%Y-%m-%d %H:%M:%S'),$status,$memory_mb,$goroutines,$uptime,$response_time" >> "$OUTPUT_DIR/metrics.csv"

    return 0
}

generate_report() {
    local elapsed=$(($(date +%s) - START_TIME))
    local hours=$((elapsed / 3600))
    local minutes=$(((elapsed % 3600) / 60))
    local success_rate=0

    if [ $TOTAL_CHECKS -gt 0 ]; then
        success_rate=$(( (TOTAL_CHECKS - FAILED_CHECKS) * 100 / TOTAL_CHECKS ))
    fi

    local report="$OUTPUT_DIR/report_$(date '+%Y%m%d_%H%M%S').txt"

    cat > "$report" << EOF
ZimaOS-Blue Soak Test Report
============================
Generated: $(date '+%Y-%m-%d %H:%M:%S')

Test Duration: ${hours}h ${minutes}m
Total Health Checks: $TOTAL_CHECKS
Failed Checks: $FAILED_CHECKS
Success Rate: ${success_rate}%

Resource Peaks:
  Max Memory: ${MAX_MEMORY} MB
  Max Goroutines: $MAX_GOROUTINES

Recent Errors (last 10):
EOF

    # Add last 10 errors
    local error_count=${#ERRORS[@]}
    local start_idx=$((error_count > 10 ? error_count - 10 : 0))
    for ((i=start_idx; i<error_count; i++)); do
        echo "  - ${ERRORS[$i]}" >> "$report"
    done

    if [ $error_count -eq 0 ]; then
        echo "  (none)" >> "$report"
    fi

    log_info "Report generated: $report"

    # Print summary to console
    echo ""
    echo "=============================================="
    echo "  Soak Test Status (${hours}h ${minutes}m)"
    echo "=============================================="
    echo "  Checks: $TOTAL_CHECKS (${success_rate}% success)"
    echo "  Max Memory: ${MAX_MEMORY} MB"
    echo "  Max Goroutines: $MAX_GOROUTINES"
    echo "=============================================="
    echo ""
}

check_for_leaks() {
    # Simple leak detection based on trends
    local recent_memory=$(tail -10 "$OUTPUT_DIR/metrics.csv" | cut -d',' -f3 | grep -v "memory_mb" | awk '{sum+=$1} END {print int(sum/NR)}')
    local early_memory=$(head -20 "$OUTPUT_DIR/metrics.csv" | tail -10 | cut -d',' -f3 | grep -v "memory_mb" | awk '{sum+=$1} END {print int(sum/NR)}')

    if [ -n "$recent_memory" ] && [ -n "$early_memory" ] && [ "$early_memory" -gt 0 ]; then
        local growth=$(( (recent_memory - early_memory) * 100 / early_memory ))
        if [ "$growth" -gt 50 ]; then
            log_warn "Potential memory leak detected: ${growth}% growth"
            ERRORS+=("$(date '+%Y-%m-%d %H:%M:%S'): Potential memory leak - ${growth}% growth")
        fi
    fi

    # Check goroutine growth
    local recent_goroutines=$(tail -10 "$OUTPUT_DIR/metrics.csv" | cut -d',' -f4 | grep -v "goroutines" | awk '{sum+=$1} END {print int(sum/NR)}')
    local early_goroutines=$(head -20 "$OUTPUT_DIR/metrics.csv" | tail -10 | cut -d',' -f4 | grep -v "goroutines" | awk '{sum+=$1} END {print int(sum/NR)}')

    if [ -n "$recent_goroutines" ] && [ -n "$early_goroutines" ] && [ "$early_goroutines" -gt 0 ]; then
        local goroutine_growth=$(( (recent_goroutines - early_goroutines) * 100 / early_goroutines ))
        if [ "$goroutine_growth" -gt 100 ]; then
            log_warn "Potential goroutine leak detected: ${goroutine_growth}% growth"
            ERRORS+=("$(date '+%Y-%m-%d %H:%M:%S'): Potential goroutine leak - ${goroutine_growth}% growth")
        fi
    fi
}

cleanup() {
    log_info "Soak test interrupted, generating final report..."
    generate_report
    exit 0
}

main() {
    trap cleanup SIGINT SIGTERM

    setup

    local end_time=$((START_TIME + DURATION_HOURS * 3600))
    local last_report=$START_TIME

    while [ $(date +%s) -lt $end_time ]; do
        # Health check
        if check_health; then
            log_success "Health check passed (${TOTAL_CHECKS} total)"
        else
            log_error "Health check failed (${FAILED_CHECKS}/${TOTAL_CHECKS} failures)"
        fi

        # Periodic report
        local now=$(date +%s)
        if [ $((now - last_report)) -ge $REPORT_INTERVAL ]; then
            generate_report
            check_for_leaks
            last_report=$now
        fi

        sleep "$CHECK_INTERVAL"
    done

    log_info "Soak test completed"
    generate_report

    # Final status
    if [ $FAILED_CHECKS -eq 0 ]; then
        log_success "All health checks passed!"
        exit 0
    else
        local failure_rate=$((FAILED_CHECKS * 100 / TOTAL_CHECKS))
        if [ $failure_rate -lt 1 ]; then
            log_warn "Soak test completed with ${failure_rate}% failure rate"
            exit 0
        else
            log_error "Soak test failed with ${failure_rate}% failure rate"
            exit 1
        fi
    fi
}

main "$@"
