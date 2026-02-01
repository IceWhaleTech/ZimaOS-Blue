#!/bin/bash
#
# ZimaOS-Echo Chaos Testing Script
# Tests system resilience under various failure conditions
#

set -e

# Configuration
SERVICE_NAME="${SERVICE_NAME:-zimaos-echo}"
HEALTH_URL="${HEALTH_URL:-http://localhost:23456/health}"
TEST_DURATION="${TEST_DURATION:-60}"  # seconds per test
RECOVERY_TIMEOUT="${RECOVERY_TIMEOUT:-30}"  # seconds to wait for recovery

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Results
TESTS_PASSED=0
TESTS_FAILED=0
RESULTS=()

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

check_health() {
    local timeout=${1:-5}
    curl -sf --max-time "$timeout" "$HEALTH_URL" > /dev/null 2>&1
}

wait_for_recovery() {
    local timeout=$RECOVERY_TIMEOUT
    local start=$(date +%s)

    while [ $(($(date +%s) - start)) -lt $timeout ]; do
        if check_health 2; then
            return 0
        fi
        sleep 1
    done
    return 1
}

record_result() {
    local test_name=$1
    local result=$2
    local details=$3

    if [ "$result" = "PASS" ]; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        log_success "$test_name"
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        log_fail "$test_name: $details"
    fi

    RESULTS+=("$test_name: $result - $details")
}

# Test 1: Process Kill Recovery
test_process_kill() {
    log_info "Test: Process Kill Recovery"

    if ! check_health; then
        record_result "Process Kill Recovery" "SKIP" "Service not running"
        return
    fi

    # Get PID
    local pid=$(pgrep -f "$SERVICE_NAME" | head -1)
    if [ -z "$pid" ]; then
        record_result "Process Kill Recovery" "SKIP" "Could not find process"
        return
    fi

    # Kill process
    log_info "Killing process $pid..."
    kill -9 "$pid" 2>/dev/null || true

    sleep 2

    # Wait for systemd to restart
    if wait_for_recovery; then
        record_result "Process Kill Recovery" "PASS" "Recovered in ${RECOVERY_TIMEOUT}s"
    else
        record_result "Process Kill Recovery" "FAIL" "Did not recover within ${RECOVERY_TIMEOUT}s"
    fi
}

# Test 2: SIGTERM Graceful Shutdown
test_sigterm() {
    log_info "Test: SIGTERM Graceful Shutdown"

    if ! check_health; then
        record_result "SIGTERM Graceful Shutdown" "SKIP" "Service not running"
        return
    fi

    local pid=$(pgrep -f "$SERVICE_NAME" | head -1)
    if [ -z "$pid" ]; then
        record_result "SIGTERM Graceful Shutdown" "SKIP" "Could not find process"
        return
    fi

    # Send SIGTERM
    log_info "Sending SIGTERM to $pid..."
    kill -15 "$pid" 2>/dev/null || true

    sleep 5

    # Wait for restart
    if wait_for_recovery; then
        record_result "SIGTERM Graceful Shutdown" "PASS" "Graceful shutdown and recovery"
    else
        record_result "SIGTERM Graceful Shutdown" "FAIL" "Did not recover"
    fi
}

# Test 3: SIGHUP Config Reload
test_sighup() {
    log_info "Test: SIGHUP Config Reload"

    if ! check_health; then
        record_result "SIGHUP Config Reload" "SKIP" "Service not running"
        return
    fi

    local pid=$(pgrep -f "$SERVICE_NAME" | head -1)
    if [ -z "$pid" ]; then
        record_result "SIGHUP Config Reload" "SKIP" "Could not find process"
        return
    fi

    # Send SIGHUP
    log_info "Sending SIGHUP to $pid..."
    kill -HUP "$pid" 2>/dev/null || true

    sleep 2

    # Check still healthy
    if check_health; then
        record_result "SIGHUP Config Reload" "PASS" "Service remained healthy after reload"
    else
        record_result "SIGHUP Config Reload" "FAIL" "Service became unhealthy after reload"
    fi
}

# Test 4: Memory Pressure
test_memory_pressure() {
    log_info "Test: Memory Pressure"

    if ! command -v stress &> /dev/null; then
        record_result "Memory Pressure" "SKIP" "stress tool not installed"
        return
    fi

    if ! check_health; then
        record_result "Memory Pressure" "SKIP" "Service not running"
        return
    fi

    # Apply memory pressure
    log_info "Applying memory pressure for ${TEST_DURATION}s..."
    stress --vm 2 --vm-bytes 256M --timeout "${TEST_DURATION}s" &
    local stress_pid=$!

    # Monitor health during stress
    local failures=0
    local checks=0
    local start=$(date +%s)

    while [ $(($(date +%s) - start)) -lt $TEST_DURATION ]; do
        checks=$((checks + 1))
        if ! check_health 5; then
            failures=$((failures + 1))
        fi
        sleep 5
    done

    # Cleanup
    kill $stress_pid 2>/dev/null || true
    wait $stress_pid 2>/dev/null || true

    sleep 2

    # Check recovery
    if check_health && [ $failures -lt $((checks / 2)) ]; then
        record_result "Memory Pressure" "PASS" "Survived with $failures/$checks health check failures"
    else
        record_result "Memory Pressure" "FAIL" "Too many failures: $failures/$checks"
    fi
}

# Test 5: CPU Starvation
test_cpu_starvation() {
    log_info "Test: CPU Starvation"

    if ! command -v stress &> /dev/null; then
        record_result "CPU Starvation" "SKIP" "stress tool not installed"
        return
    fi

    if ! check_health; then
        record_result "CPU Starvation" "SKIP" "Service not running"
        return
    fi

    # Apply CPU pressure
    local cpus=$(nproc)
    log_info "Applying CPU pressure ($cpus cores) for ${TEST_DURATION}s..."
    stress --cpu "$cpus" --timeout "${TEST_DURATION}s" &
    local stress_pid=$!

    # Monitor health during stress
    local failures=0
    local checks=0
    local start=$(date +%s)

    while [ $(($(date +%s) - start)) -lt $TEST_DURATION ]; do
        checks=$((checks + 1))
        if ! check_health 10; then
            failures=$((failures + 1))
        fi
        sleep 5
    done

    # Cleanup
    kill $stress_pid 2>/dev/null || true
    wait $stress_pid 2>/dev/null || true

    sleep 2

    # Check recovery
    if check_health && [ $failures -lt $((checks / 2)) ]; then
        record_result "CPU Starvation" "PASS" "Survived with $failures/$checks health check failures"
    else
        record_result "CPU Starvation" "FAIL" "Too many failures: $failures/$checks"
    fi
}

# Test 6: Rapid Restart
test_rapid_restart() {
    log_info "Test: Rapid Restart (5 times)"

    if ! systemctl is-active --quiet "$SERVICE_NAME"; then
        record_result "Rapid Restart" "SKIP" "Service not managed by systemd"
        return
    fi

    local failures=0

    for i in {1..5}; do
        log_info "Restart $i/5..."
        systemctl restart "$SERVICE_NAME"
        sleep 3

        if ! check_health; then
            failures=$((failures + 1))
        fi
    done

    if [ $failures -eq 0 ]; then
        record_result "Rapid Restart" "PASS" "All 5 restarts successful"
    else
        record_result "Rapid Restart" "FAIL" "$failures/5 restarts failed"
    fi
}

# Test 7: Network Partition Simulation
test_network_partition() {
    log_info "Test: Network Partition (iptables)"

    if ! command -v iptables &> /dev/null; then
        record_result "Network Partition" "SKIP" "iptables not available"
        return
    fi

    if [ "$EUID" -ne 0 ]; then
        record_result "Network Partition" "SKIP" "Requires root"
        return
    fi

    if ! check_health; then
        record_result "Network Partition" "SKIP" "Service not running"
        return
    fi

    # Block outgoing connections (simulate network partition)
    log_info "Blocking outgoing connections..."
    iptables -A OUTPUT -p tcp --dport 443 -j DROP
    iptables -A OUTPUT -p tcp --dport 80 -j DROP

    sleep 10

    # Check service is still responding locally
    local local_healthy=true
    if ! check_health; then
        local_healthy=false
    fi

    # Restore network
    log_info "Restoring network..."
    iptables -D OUTPUT -p tcp --dport 443 -j DROP
    iptables -D OUTPUT -p tcp --dport 80 -j DROP

    sleep 5

    if $local_healthy && check_health; then
        record_result "Network Partition" "PASS" "Service remained healthy during partition"
    else
        record_result "Network Partition" "FAIL" "Service became unhealthy"
    fi
}

# Print Summary
print_summary() {
    echo ""
    echo "=============================================="
    echo "  Chaos Test Results"
    echo "=============================================="
    echo ""

    for result in "${RESULTS[@]}"; do
        echo "  $result"
    done

    echo ""
    echo "----------------------------------------------"
    echo -e "  Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "  Failed: ${RED}$TESTS_FAILED${NC}"
    echo "----------------------------------------------"
    echo ""

    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        return 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        return 1
    fi
}

# Main
main() {
    echo ""
    echo "=============================================="
    echo "  ZimaOS-Echo Chaos Testing"
    echo "=============================================="
    echo ""
    echo "Service: $SERVICE_NAME"
    echo "Health URL: $HEALTH_URL"
    echo "Test Duration: ${TEST_DURATION}s per test"
    echo ""

    # Run tests
    test_process_kill
    test_sigterm
    test_sighup
    test_memory_pressure
    test_cpu_starvation
    test_rapid_restart
    # test_network_partition  # Uncomment if running as root

    print_summary
}

main "$@"
