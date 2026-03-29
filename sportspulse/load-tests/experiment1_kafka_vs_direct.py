"""
Experiment 1: Kafka-buffered ingestion vs Direct DB writes
==========================================================
Run against Kafka variant:
    locust -f experiment1_kafka_vs_direct.py --host http://localhost:8080 \
           --users 100 --spawn-rate 10 --run-time 60s --headless \
           --csv results/exp1_kafka

Run against Direct DB variant:
    locust -f experiment1_kafka_vs_direct.py --host http://localhost:8083 \
           --users 100 --spawn-rate 10 --run-time 60s --headless \
           --csv results/exp1_direct

Compare the two CSV outputs for throughput and latency.
"""

import uuid
import random
from locust import HttpUser, task, between, events
import csv, os, time

PLAYERS  = ["p1", "p2", "p3", "p4", "p5"]
TEAMS    = {"p1": "lakers", "p2": "warriors", "p3": "suns", "p4": "bucks", "p5": "nuggets"}
EV_TYPES = ["shot", "assist", "rebound"]
VALUES   = {"shot": [2.0, 3.0], "assist": [1.0], "rebound": [1.0]}


class EventIngestionUser(HttpUser):
    """
    Simulates a live game event stream.
    Each user fires events at a rate matching ~500-2000 events/min per user.
    """
    wait_time = between(0.05, 0.2)  # 5–20ms between requests → ~300–1200 req/min per user

    @task
    def ingest_event(self):
        player_id  = random.choice(PLAYERS)
        event_type = random.choice(EV_TYPES)
        value      = random.choice(VALUES[event_type])

        payload = {
            "event_id":   str(uuid.uuid4()),
            "player_id":  player_id,
            "team_id":    TEAMS[player_id],
            "event_type": event_type,
            "value":      value,
        }

        with self.client.post(
            "/events",
            json=payload,
            catch_response=True,
            name="POST /events",
        ) as resp:
            if resp.status_code == 202:
                resp.success()
            else:
                resp.failure(f"unexpected status {resp.status_code}: {resp.text}")