"""
Part 2 Load Tests
Test 1 - Baseline:      5 users,  2 min  → expect ~60% CPU
Test 2 - Breaking Point: 20 users, 3 min → expect ~100% CPU, degraded response times

Run:
  locust -f locustfile.py --host http://<your-alb-or-ecs-ip>:8080
  Then open http://localhost:8089 and configure users/duration.
  
  Or headless:
  locust -f locustfile.py --host http://<host>:8080 \
         --users 5 --spawn-rate 1 --run-time 2m --headless --csv baseline
"""

from locust import FastHttpUser, task, between, constant
import random

# Search terms designed to always hit (common words in product names/categories)
SEARCH_TERMS = [
    "Electronics",
    "Alpha",
    "Beta",
    "Books",
    "Product",
    "Home",
    "Sports",
    "Gamma",
    "Clothing",
    "Delta",
]


class ProductSearchUser(FastHttpUser):
    # Minimal wait time to maximize pressure
    wait_time = constant(0)  # no wait — hammer the service

    @task
    def search_products(self):
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
            f"/products/search?q={term}",
            name="/products/search",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                data = resp.json()
                # Verify we're actually checking exactly 100 products
                if data.get("products_checked") != 100:
                    resp.failure(
                        f"Expected 100 products checked, got {data.get('products_checked')}"
                    )
                else:
                    resp.success()
            else:
                resp.failure(f"HTTP {resp.status_code}")

    @task(1)  # low weight health check — simulates real traffic mix
    def health_check(self):
        self.client.get("/health", name="/health")