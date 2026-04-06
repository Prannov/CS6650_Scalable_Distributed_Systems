#!/bin/bash
set -e

DURATION=30
WORKERS=20
KEYS=20
RESULTS_DIR="results"

mkdir -p "$RESULTS_DIR"

run_loadtest() {
  local label=$1
  local writes=$2
  echo "  → writes=${writes}% reads=$((100 - writes))%"
  go run ./loadtest \
    -writes="$writes" \
    -duration="$DURATION" \
    -workers="$WORKERS" \
    -keys="$KEYS" \
    -out="${RESULTS_DIR}/${label}_${writes}w.json"
}

run_config() {
  local label=$1
  echo ""
  echo "=== $label ==="
  for writes in 1 10 50 90; do
    run_loadtest "$label" "$writes"
  done
}

switch_leader_mode() {
  local mode=$1
  echo ""
  echo "--- Switching leader mode to $mode ---"
  # Update the mode anchor in docker-compose.leader.yml
  sed -i.bak "s/x-mode: &mode .*/x-mode: \&mode $mode/" docker-compose.leader.yml
  docker compose -f docker-compose.leader.yml down --remove-orphans
  docker compose -f docker-compose.leader.yml up -d
  echo "Waiting 3s for nodes to be ready..."
  sleep 3
}

# ── Leader-Follower ──────────────────────────────────────────────────────────

# Make sure leaderless is down
docker compose -f docker-compose.leaderless.yml down --remove-orphans 2>/dev/null || true

switch_leader_mode W5R1
run_config "W5R1"

switch_leader_mode W1R5
run_config "W1R5"

switch_leader_mode W3R3
run_config "W3R3"

# ── Leaderless ───────────────────────────────────────────────────────────────

echo ""
echo "--- Switching to Leaderless ---"
docker compose -f docker-compose.leader.yml down --remove-orphans
docker compose -f docker-compose.leaderless.yml up -d
echo "Waiting 3s for nodes to be ready..."
sleep 3

# Load tester targets node1 for writes by default (localhost:8080)
run_config "Leaderless"

# ── Done ─────────────────────────────────────────────────────────────────────

echo ""
echo "=== All load tests complete ==="
echo "Results written to ./${RESULTS_DIR}/"
ls -lh "${RESULTS_DIR}/"

# Tear down
docker compose -f docker-compose.leaderless.yml down --remove-orphans