from locust import HttpUser, task, between
import random

SAMPLE_ITEMS = [
    {"product_id": "SHOE-001", "quantity": 1, "price": 89.99},
    {"product_id": "SHIRT-42",  "quantity": 2, "price": 24.99},
    {"product_id": "HAT-007",  "quantity": 1, "price": 14.99},
]

def order_payload():
    return {
        "customer_id": random.randint(1, 10000),
        "items": random.sample(SAMPLE_ITEMS, k=random.randint(1, 2)),
    }

class SyncNormalUser(HttpUser):
    """Phase 1 — Normal operations: 5 users, 30s"""
    wait_time = between(0.1, 0.5)
    host = "http://orders-alb-1470263549.us-east-1.elb.amazonaws.com"

    @task
    def place_sync_order(self):
        self.client.post("/orders/sync", json=order_payload(), timeout=15)


class SyncFlashUser(HttpUser):
    """Phase 2 — Flash sale sync: 20 users, 60s"""
    wait_time = between(0.1, 0.5)
    host = "http://orders-alb-1470263549.us-east-1.elb.amazonaws.com"

    @task
    def place_sync_order(self):
        self.client.post("/orders/sync", json=order_payload(), timeout=15)


class AsyncFlashUser(HttpUser):
    """Phase 3 — Flash sale async: 20 users, 60s"""
    wait_time = between(0.1, 0.5)
    host = "http://orders-alb-1470263549.us-east-1.elb.amazonaws.com"

    @task
    def place_async_order(self):
        with self.client.post(
            "/orders/async",
            json=order_payload(),
            catch_response=True,
            timeout=5,
        ) as resp:
            if resp.status_code == 202:
                resp.success()
            else:
                resp.failure(f"Expected 202, got {resp.status_code}")