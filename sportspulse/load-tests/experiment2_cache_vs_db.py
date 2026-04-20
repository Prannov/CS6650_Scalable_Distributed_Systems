"""
Experiment 2: Redis Cache vs Direct DB Reads
=============================================
Tests three configurations under increasing read load:

Config A — No cache, direct Postgres (query-svc with cache disabled)
    locust -f experiment2_cache_vs_db.py --host http://localhost:8082 \
           --users 100 --spawn-rate 10 --run-time 60s --headless \
           --csv results/exp2_nocache_100

Config B — Redis cache enabled (current query-svc)
    locust -f experiment2_cache_vs_db.py --host http://localhost:8081 \
           --users 100 --spawn-rate 10 --run-time 60s --headless \
           --csv results/exp2_cache_100

Run at 100, 500, 1000 users for each config.
"""

import random
from locust import HttpUser, task, between

PLAYERS = ["p1", "p2", "p3", "p4", "p5"]


class FanQueryUser(HttpUser):
    """
    Simulates fans querying player stats and leaderboard.
    80% single player stats, 20% leaderboard (mimics real fan behavior).
    """
    wait_time = between(0.05, 0.1)  # 10–20 req/s per user

    @task(4)
    def query_player_stats(self):
        player_id = random.choice(PLAYERS)
        with self.client.get(
            f"/stats?player_id={player_id}",
            catch_response=True,
            name="GET /stats",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"unexpected status {resp.status_code}")

    @task(1)
    def query_leaderboard(self):
        with self.client.get(
            "/leaderboard",
            catch_response=True,
            name="GET /leaderboard",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"unexpected status {resp.status_code}")