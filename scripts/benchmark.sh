#!/bin/bash
set -e

# ============================================================
# BDopsFlow Performance Benchmark Script
# Tool: hey (Go HTTP load generator)
# Usage: ./scripts/benchmark.sh [BASE_URL] [TOKEN]
# Example: ./scripts/benchmark.sh http://localhost:8080 "your-jwt-token"
# ============================================================

HEY="${HOME}/go/bin/hey"
BASE_URL="${1:-http://localhost:8080}"
TOKEN="${2:-}"

REQUESTS=500
CONCURRENCY=20

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

RESULTS_DIR="/tmp/bdopsflow_benchmark_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

print_header() {
  echo ""
  echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
  echo -e "${BLUE}║${NC}  ${BOLD}BDopsFlow Performance Benchmark${NC}                              ${BLUE}║${NC}"
  echo -e "${BLUE}╠══════════════════════════════════════════════════════════════╣${NC}"
  echo -e "${BLUE}║${NC}  Target: ${CYAN}${BASE_URL}${NC}"
  printf "${BLUE}║${NC}  Requests: %-6s  Concurrency: %-6s                          ${BLUE}║${NC}\n" "$REQUESTS" "$CONCURRENCY"
  echo -e "${BLUE}║${NC}  Tool: hey                                                  ${BLUE}║${NC}"
  echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
  echo ""
}

check_prerequisites() {
  if ! command -v "$HEY" &>/dev/null; then
    echo -e "${RED}Error: hey not found. Install: go install github.com/rakyll/hey@latest${NC}"
    exit 1
  fi

  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/health" 2>/dev/null || echo "000")
  if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}Error: Server not responding at ${BASE_URL}/health (HTTP ${HTTP_CODE})${NC}"
    echo -e "${YELLOW}Please start the server first${NC}"
    exit 1
  fi
  echo -e "${GREEN}✓ Server is running (HTTP 200)${NC}"
}

extract_metrics() {
  local raw_file="$1"
  local qps=$(grep "Requests/sec" "$raw_file" | awk '{print $2}')
  local avg=$(grep "Average" "$raw_file" | head -1 | awk '{print $2}')
  local fastest=$(grep "Fastest" "$raw_file" | awk '{print $2}')
  local slowest=$(grep "Slowest" "$raw_file" | awk '{print $2}')
  local p50=$(grep "50%" "$raw_file" | tail -1 | awk '{print $2}')
  local p90=$(grep "90%" "$raw_file" | tail -1 | awk '{print $2}')
  local p95=$(grep "95%" "$raw_file" | tail -1 | awk '{print $2}')
  local p99=$(grep "99%" "$raw_file" | tail -1 | awk '{print $2}')
  local total_time=$(grep "Total:" "$raw_file" | awk '{print $2}')
  local success=$(grep "2xx," "$raw_file" | awk -F'[(), ]+' '{for(i=1;i<=NF;i++) if($i=="2xx,") print $(i+1)}')
  local errors=$(grep -E "4xx,|5xx," "$raw_file" | awk -F'[(), ]+' '{for(i=1;i<=NF;i++) if($i=="4xx," || $i=="5xx,") print $(i+1)}' | paste -sd+ - | bc 2>/dev/null || echo "0")

  echo "${qps:-N/A}|${avg:-N/A}|${fastest:-N/A}|${slowest:-N/A}|${p50:-N/A}|${p90:-N/A}|${p95:-N/A}|${p99:-N/A}|${total_time:-N/A}|${success:-0}|${errors:-0}"
}

run_benchmark() {
  local name="$1"
  local method="$2"
  local url="$3"
  local body="$4"

  echo -e "\n${BOLD}${CYAN}▶ ${name}${NC}"
  echo -e "  ${YELLOW}URL: ${url}${NC}"

  local raw_file="${RESULTS_DIR}/${name}.raw"

  if [ "$method" = "GET" ]; then
    if [ -n "$TOKEN" ]; then
      "$HEY" -n "$REQUESTS" -c "$CONCURRENCY" -m GET \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        "$url" > "$raw_file" 2>&1
    else
      "$HEY" -n "$REQUESTS" -c "$CONCURRENCY" -m GET \
        -H "Content-Type: application/json" \
        "$url" > "$raw_file" 2>&1
    fi
  else
    if [ -n "$TOKEN" ]; then
      "$HEY" -n "$REQUESTS" -c "$CONCURRENCY" -m "$method" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        -d "$body" \
        "$url" > "$raw_file" 2>&1
    else
      "$HEY" -n "$REQUESTS" -c "$CONCURRENCY" -m "$method" \
        -H "Content-Type: application/json" \
        -d "$body" \
        "$url" > "$raw_file" 2>&1
    fi
  fi

  local metrics=$(extract_metrics "$raw_file")
  IFS='|' read -r qps avg fastest slowest p50 p90 p95 p99 total_time success errors <<< "$metrics"

  printf "  ${GREEN}QPS: %-10s${NC}  Avg: %-8s  P50: %-8s  P95: %-8s  P99: %-8s  Success: %-5s  Errors: %-3s\n" \
    "$qps" "$avg" "$p50" "$p95" "$p99" "$success" "$errors"
}

print_summary_table() {
  echo ""
  echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗${NC}"
  echo -e "${BLUE}║${NC} ${BOLD}Performance Summary${NC}                                                                                                  ${BLUE}║${NC}"
  echo -e "${BLUE}╠══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╣${NC}"
  printf "${BLUE}║${NC} %-28s │ %10s │ %8s │ %8s │ %8s │ %8s │ %7s │ %6s ${BLUE}║${NC}\n" \
    "API Endpoint" "QPS" "Avg(ms)" "P50(ms)" "P95(ms)" "P99(ms)" "Success" "Errors"
  echo -e "${BLUE}╠══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╣${NC}"

  for raw_file in "${RESULTS_DIR}"/*.raw; do
    [ -f "$raw_file" ] || continue
    local name=$(basename "$raw_file" .raw)
    local metrics=$(extract_metrics "$raw_file")
    IFS='|' read -r qps avg fastest slowest p50 p90 p95 p99 total_time success errors <<< "$metrics"

    local qps_num=$(echo "$qps" | sed 's/[a-z]*//g')
    local color="${GREEN}"
    if command -v bc &>/dev/null; then
      if echo "$qps_num < 100" | bc -l 2>/dev/null | grep -q 1; then
        color="${RED}"
      elif echo "$qps_num < 500" | bc -l 2>/dev/null | grep -q 1; then
        color="${YELLOW}"
      fi
    fi

    printf "${BLUE}║${NC} %-28s │ ${color}%10s${NC} │ %8s │ %8s │ %8s │ %8s │ %7s │ %6s ${BLUE}║${NC}\n" \
      "$name" "$qps" "$avg" "$p50" "$p95" "$p99" "$success" "$errors"
  done

  echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╝${NC}"
}

print_grade() {
  echo ""
  echo -e "${BOLD}Performance Grade:${NC}"
  echo -e "${BLUE}──────────────────${NC}"

  local total_qps=0
  local count=0
  for raw_file in "${RESULTS_DIR}"/*.raw; do
    [ -f "$raw_file" ] || continue
    local qps=$(grep "Requests/sec" "$raw_file" | awk '{print $2}' | sed 's/[a-z]*//g')
    if [ -n "$qps" ]; then
      total_qps=$(echo "$total_qps + $qps" | bc 2>/dev/null || echo "$total_qps")
      count=$((count + 1))
    fi
  done

  if [ $count -gt 0 ]; then
    local avg_qps=$(echo "scale=1; $total_qps / $count" | bc 2>/dev/null || echo "N/A")
    local grade="F"
    local grade_color="${RED}"
    if command -v bc &>/dev/null; then
      if echo "$avg_qps >= 2000" | bc -l 2>/dev/null | grep -q 1; then
        grade="A"; grade_color="${GREEN}"
      elif echo "$avg_qps >= 1000" | bc -l 2>/dev/null | grep -q 1; then
        grade="B"; grade_color="${CYAN}"
      elif echo "$avg_qps >= 500" | bc -l 2>/dev/null | grep -q 1; then
        grade="C"; grade_color="${YELLOW}"
      elif echo "$avg_qps >= 100" | bc -l 2>/dev/null | grep -q 1; then
        grade="D"; grade_color="${YELLOW}"
      fi
    fi
    echo -e "  Average QPS: ${BOLD}${avg_qps}${NC}  →  Grade: ${grade_color}${BOLD}${grade}${NC}"
  fi

  echo -e "\n  ${CYAN}QPS Grading:${NC}"
  echo -e "  ${GREEN}A${NC} ≥ 2000 QPS  |  ${CYAN}B${NC} ≥ 1000 QPS  |  ${YELLOW}C${NC} ≥ 500 QPS  |  ${YELLOW}D${NC} ≥ 100 QPS  |  ${RED}F${NC} < 100 QPS"
  echo ""
}

# ============================================================
# Main
# ============================================================

print_header
check_prerequisites

echo -e "${BOLD}Starting benchmarks...${NC}"
echo -e "${YELLOW}Each test: ${REQUESTS} requests, ${CONCURRENCY} concurrent connections${NC}\n"

# ---- Unauthenticated APIs ----
echo -e "\n${BOLD}━━━ Unauthenticated APIs ━━━${NC}"

run_benchmark "health_check" "GET" "${BASE_URL}/health"

# ---- Authenticated APIs ----
if [ -z "$TOKEN" ]; then
  echo -e "\n${YELLOW}⚠ No TOKEN provided. Skipping authenticated API tests.${NC}"
  echo -e "${YELLOW}  Usage: $0 [BASE_URL] [JWT_TOKEN]${NC}"
  echo -e "${YELLOW}  Get token: curl -s ${BASE_URL}/api/auth/login -X POST -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin\"}' | jq -r '.data.token'${NC}"
  echo ""
  print_summary_table
  print_grade
  exit 0
fi

# ---- Read-heavy APIs (most common) ----
echo -e "\n${BOLD}━━━ Read APIs (High Frequency) ━━━${NC}"

run_benchmark "task_list" "GET" "${BASE_URL}/api/tasks?page=1&page_size=20"

run_benchmark "executor_list" "GET" "${BASE_URL}/api/executors"

run_benchmark "log_list" "GET" "${BASE_URL}/api/logs?page=1&page_size=20"

run_benchmark "log_stats" "GET" "${BASE_URL}/api/logs/stats"

run_benchmark "dashboard_stats" "GET" "${BASE_URL}/api/dashboard/stats"

run_benchmark "dashboard_trends" "GET" "${BASE_URL}/api/dashboard/trends"

run_benchmark "dashboard_health" "GET" "${BASE_URL}/api/dashboard/health"

run_benchmark "scheduler_status" "GET" "${BASE_URL}/api/dashboard/scheduler/status"

# ---- Admin APIs ----
echo -e "\n${BOLD}━━━ Admin APIs ━━━${NC}"

run_benchmark "user_list" "GET" "${BASE_URL}/api/admin/users"

run_benchmark "role_list" "GET" "${BASE_URL}/api/admin/roles"

run_benchmark "domain_list" "GET" "${BASE_URL}/api/admin/domains"

run_benchmark "audit_log_list" "GET" "${BASE_URL}/api/admin/audit-logs?page=1&page_size=20"

# ---- Datasource APIs ----
echo -e "\n${BOLD}━━━ Datasource APIs ━━━${NC}"

run_benchmark "datasource_list" "GET" "${BASE_URL}/api/datasources"

run_benchmark "datasource_types" "GET" "${BASE_URL}/api/datasources/types"

# ---- Webhook APIs ----
echo -e "\n${BOLD}━━━ Webhook APIs ━━━${NC}"

run_benchmark "webhook_list" "GET" "${BASE_URL}/api/webhooks"

# ---- Write APIs (lower frequency) ----
echo -e "\n${BOLD}━━━ Write APIs (Lower Frequency) ━━━${NC}"

run_benchmark "task_create" "POST" "${BASE_URL}/api/tasks" \
  '{"name":"bench-test","type":"http","config":"{\"url\":\"http://localhost:8080/health\",\"method\":\"GET\",\"timeout\":5}","timeout_seconds":60,"retry_count":0,"retry_interval":5,"is_enabled":false}'

# ---- Query History API ----
echo -e "\n${BOLD}━━━ Query History APIs ━━━${NC}"

run_benchmark "query_history" "GET" "${BASE_URL}/api/query/history?page=1&page_size=20"

run_benchmark "saved_sql_list" "GET" "${BASE_URL}/api/query/saved?page=1&page_size=20"

# ---- Summary ----
print_summary_table
print_grade

echo -e "${CYAN}Raw results saved to: ${RESULTS_DIR}/${NC}"
echo -e "${CYAN}To view details: cat ${RESULTS_DIR}/<api_name>.raw${NC}"
echo ""
