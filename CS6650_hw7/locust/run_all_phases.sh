#!/bin/bash
set -e

ALB="http://orders-alb-1470263549.us-east-1.elb.amazonaws.com"
mkdir -p reports

echo "======================================"
echo " Phase 1: Normal Sync (5 users, 30s)"
echo "======================================"
locust -f locustfile.py SyncNormalUser \
  --users 5 --spawn-rate 1 --run-time 30s --headless \
  -H $ALB \
  --csv=reports/phase1_sync_normal \
  --html=reports/phase1_sync_normal.html

echo ""
echo "======================================"
echo " Phase 2: Flash Sale Sync (20 users, 60s)"
echo "======================================"
locust -f locustfile.py SyncFlashUser \
  --users 20 --spawn-rate 10 --run-time 60s --headless \
  -H $ALB \
  --csv=reports/phase2_sync_flash \
  --html=reports/phase2_sync_flash.html

echo ""
echo "======================================"
echo " Phase 3: Flash Sale Async (20 users, 60s)"
echo "======================================"
locust -f locustfile.py AsyncFlashUser \
  --users 20 --spawn-rate 10 --run-time 60s --headless \
  -H $ALB \
  --csv=reports/phase3_async_flash \
  --html=reports/phase3_async_flash.html

echo ""
echo "======================================"
echo " All phases complete. Reports saved to ./reports/"
echo " Files per phase:"
echo "   *_stats.csv        — per-endpoint summary"
echo "   *_stats_history.csv — metrics over time"
echo "   *_failures.csv     — error breakdown"
echo "   *.html             — full visual report"
echo "======================================"