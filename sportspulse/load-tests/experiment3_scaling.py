"""
Experiment 3: ECS Horizontal Scaling
=====================================
Run the same fixed load against query-svc at 1, 2, 4, 8 ECS tasks.

Scale tasks first, then run:
    locust -f experiment3_scaling.py \
           --host http://sportspulse-alb-33524114.us-east-1.elb.amazonaws.com:8081 \
           --users 300 --spawn-rate 30 --run-time 60s --headless \
           --csv results/exp3_tasks_1

Change --csv filename for each task count run.
"""

import random
from locust import HttpUser, task, between

PLAYERS = ["p1", "p2", "p3", "p4", "p5"]


class FanQueryUser(HttpUser):
    wait_time = between(0.05, 0.1)

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
                resp.failure(f"status {resp.status_code}")

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
                resp.failure(f"status {resp.status_code}")