from locust import HttpUser, task, between
import json
import random

class ProductAPIUser(HttpUser):
    wait_time = between(1, 3)  # Wait 1-3 seconds between requests
    
    def on_start(self):
        """Called when a simulated user starts"""
        # Add some initial products
        for i in range(1, 6):
            self.add_product(i)
    
    def add_product(self, product_id):
        """Helper method to add a product"""
        product_data = {
            "sku": f"SKU-{product_id}",
            "manufacturer": f"Manufacturer-{random.randint(1, 10)}",
            "category_id": random.randint(1, 20),
            "weight": random.randint(100, 5000),
            "some_other_id": random.randint(1, 100)
        }
        
        self.client.post(
            f"/v1/products/{product_id}/details",
            json=product_data,
            name="/v1/products/[id]/details (POST)"
        )
    
    @task(10)  # Weight 10 - this runs 10x more often than task(1)
    def get_product(self):
        """GET request - should be most common in real world"""
        product_id = random.randint(1, 100)
        with self.client.get(
            f"/v1/products/{product_id}",
            name="/v1/products/[id] (GET)",
            catch_response=True
        ) as response:
            if response.status_code == 404:
                # Not found is expected for non-existent products
                response.success()
    
    @task(2)  # Weight 2
    def add_product_task(self):
        """POST request to add/update products"""
        product_id = random.randint(1, 100)
        self.add_product(product_id)
    
    @task(1)  # Weight 1
    def health_check(self):
        """Health check endpoint"""
        self.client.get("/v1/health", name="/v1/health")
    
    @task(1)
    def get_nonexistent_product(self):
        """Test 404 handling"""
        product_id = random.randint(1000, 9999)
        with self.client.get(
            f"/v1/products/{product_id}",
            name="/v1/products/[id] (GET 404)",
            catch_response=True
        ) as response:
            if response.status_code == 404:
                response.success()
            else:
                response.failure(f"Expected 404, got {response.status_code}")
    
    @task(1)
    def invalid_product_id(self):
        """Test 400 handling with invalid product ID"""
        with self.client.get(
            "/v1/products/invalid",
            name="/v1/products/invalid (GET 400)",
            catch_response=True
        ) as response:
            if response.status_code == 400:
                response.success()
            else:
                response.failure(f"Expected 400, got {response.status_code}")


class FastProductAPIUser(HttpUser):
    """Using FastHttpUser for comparison"""
    wait_time = between(0.5, 1.5)
    
    @task(10)
    def get_product(self):
        product_id = random.randint(1, 100)
        self.client.get(
            f"/v1/products/{product_id}",
            name="/v1/products/[id] (FastHttpUser)"
        )
    
    @task(2)
    def add_product(self):
        product_id = random.randint(1, 100)
        product_data = {
            "sku": f"SKU-{product_id}",
            "manufacturer": f"Manufacturer-{random.randint(1, 10)}",
            "category_id": random.randint(1, 20),
            "weight": random.randint(100, 5000),
            "some_other_id": random.randint(1, 100)
        }
        self.client.post(
            f"/v1/products/{product_id}/details",
            json=product_data,
            name="/v1/products/[id]/details (FastHttpUser)"
        )