from locust import FastHttpUser, task, between

class ApiUser(FastHttpUser):
    wait_time = between(0, 0)

    @task(3)
    def get_albums(self):
        with self.client.get("/albums", name="GET /albums", catch_response=True) as r:
            if r.status_code != 200:
                r.failure(f"{r.status_code}: {r.text}")

    @task(1)
    def post_albums(self):
        payload = {"title": "locust-title", "artist": "locust-artist", "year": 2026}
        with self.client.post("/albums", json=payload, name="POST /albums", catch_response=True) as r:
            if r.status_code not in (200, 201):
                r.failure(f"{r.status_code}: {r.text}")
