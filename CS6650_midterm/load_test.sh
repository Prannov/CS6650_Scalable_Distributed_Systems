#!/bin/bash
# Usage: ./load_test.sh [concurrency] [duration_seconds]
# macOS compatible — uses python3 for millisecond timestamps

SERVICE_URL=${SERVICE_URL:-http://localhost:8080}
CONCURRENCY=${1:-50}
DURATION=${2:-30}
RESULTS_FILE=/tmp/load_results_$$.txt
> "$RESULTS_FILE"

# macOS doesn't support date +%s%3N, use python3 instead
ms() { python3 -c "import time; print(int(time.time()*1000))"; }

echo "🚀 Load test: ${CONCURRENCY} concurrent users for ${DURATION}s"
echo "   Target: ${SERVICE_URL}/search?q=sneakers"
echo ""

START_TIME=$(date +%s)
END_TIME=$(( START_TIME + DURATION ))

fire() {
  local out="$1"
  local t0
  t0=$(python3 -c "import time; print(int(time.time()*1000))")
  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 6 \
    "${SERVICE_URL}/search?q=sneakers")
  local t1
  t1=$(python3 -c "import time; print(int(time.time()*1000))")
  echo "${status} $(( t1 - t0 ))" >> "$out"
}
export -f fire
export SERVICE_URL

while [ "$(date +%s)" -lt "$END_TIME" ]; do
  # Spawn background jobs, each writing directly to results file
  for i in $(seq 1 "$CONCURRENCY"); do
    fire "$RESULTS_FILE" &
  done
  wait

  TOTAL=$(wc -l < "$RESULTS_FILE" | tr -d ' ')
  GOROUTINES=$(curl -s --max-time 1 "${SERVICE_URL}/metrics" \
    | grep -o '"goroutines":[0-9]*' | grep -o '[0-9]*' || echo "?")
  CB=$(curl -s --max-time 1 "${SERVICE_URL}/metrics" \
    | grep -o '"circuit_breaker":"[^"]*"' | sed 's/.*:"\(.*\)"/cb:\1/' || echo "")

  echo "  batch done | total: ${TOTAL} requests | goroutines: ${GOROUTINES} ${CB}"
done

echo ""

# ── Summary ──────────────────────────────────────────
TOTAL=$(wc -l < "$RESULTS_FILE" | tr -d ' ')

if [ "$TOTAL" -eq 0 ]; then
  echo "No results collected. Is the service running at ${SERVICE_URL}?"
  echo "Try: curl ${SERVICE_URL}/health"
  rm -f "$RESULTS_FILE"
  exit 1
fi

SUCCESS=$(grep -c "^200" "$RESULTS_FILE" 2>/dev/null || echo 0)
ERRORS=$(( TOTAL - SUCCESS ))
ERROR_PCT=$(( ERRORS * 100 / TOTAL ))

AVG_LAT=$(awk '{s+=$2; n++} END {printf "%d", (n>0?s/n:0)}' "$RESULTS_FILE")
P50=$(awk '{print $2}' "$RESULTS_FILE" | sort -n | awk '{a[NR]=$1} END {print a[int(NR*0.50)+1]}')
P95=$(awk '{print $2}' "$RESULTS_FILE" | sort -n | awk '{a[NR]=$1} END {print a[int(NR*0.95)+1]}')
P99=$(awk '{print $2}' "$RESULTS_FILE" | sort -n | awk '{a[NR]=$1} END {print a[int(NR*0.99)+1]}')

echo "════════════════════════════════"
echo "  📊 Results"
echo "════════════════════════════════"
echo "  Total requests : ${TOTAL}"
echo "  Successes      : ${SUCCESS}"
echo "  Errors         : ${ERRORS} (${ERROR_PCT}%)"
echo "  Avg latency    : ${AVG_LAT}ms"
echo "  p50 latency    : ${P50}ms"
echo "  p95 latency    : ${P95}ms"
echo "  p99 latency    : ${P99}ms"
echo "════════════════════════════════"

rm -f "$RESULTS_FILE"