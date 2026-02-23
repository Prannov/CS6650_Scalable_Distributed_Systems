#!/bin/bash
set -e

# Pass your host as an argument, e.g.:
#   ./scripts/run_tests.sh http://1.2.3.4:8080          (Part 2, direct IP)
#   ./scripts/run_tests.sh http://my-alb-dns.amazonaws.com  (Part 3, ALB)

HOST=${1:?"Usage: $0 <host>  e.g. http://1.2.3.4:8080"}

echo "======================================"
echo " Target: $HOST"
echo "======================================"

# ── Verify service is up ──────────────────
echo "→ Health check..."
curl -sf "$HOST/health" | python3 -m json.tool
echo ""

# ── Part 2: Test 1 — Baseline (5 users, 2 min) ──
echo "→ Running BASELINE test (5 users, 2 min)..."
locust -f locustfile.py \
  --host "$HOST" \
  --users 5 \
  --spawn-rate 1 \
  --run-time 2m \
  --headless \
  --csv results/part2_baseline \
  --html results/part2_baseline.html
echo "✅ Baseline complete. See results/part2_baseline.html"

sleep 10  # cool-down

# ── Part 2: Test 2 — Breaking Point (20 users, 3 min) ──
echo "→ Running BREAKING POINT test (20 users, 3 min)..."
locust -f locustfile.py \
  --host "$HOST" \
  --users 20 \
  --spawn-rate 2 \
  --run-time 3m \
  --headless \
  --csv results/part2_break \
  --html results/part2_break.html
echo "✅ Breaking point test complete. See results/part2_break.html"

echo ""
echo "======================================"
echo " Summary CSVs saved to results/"
echo " Open the .html files for charts"
echo "======================================"