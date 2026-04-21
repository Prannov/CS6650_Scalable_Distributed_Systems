# 🏆 SportsPulse

**A distributed player performance tracking and analytics platform built for flash-sale scale.**

SportsPulse solves the dual scaling challenge of live sports platforms: handling thousands of live game events per minute during a match, while simultaneously serving millions of fan queries on player stats, leaderboards, and performance trends — without either path degrading the other.

Built for CS6650 – Scalable Distributed Systems @ Northeastern University.

**Team:** Prannov Jamadagni & Eroniction Presley

---

## Architecture

```
                        WRITE PATH
┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐    ┌──────────────┐
│  Game Event │───▶│  event-svc  │───▶│  Kafka              │───▶│ stats-worker │
│  (POST)     │    │  :8080      │    │  game-events topic  │    │              │
└─────────────┘    └─────────────┘    │  3 partitions       │    └──────┬───────┘
                                      └─────────────────────┘           │
                                                                        ▼
                        READ PATH                              ┌──────────────┐
┌─────────────┐    ┌─────────────┐    ┌──────────────┐         │  PostgreSQL  │
│  Fan Query  │───▶│  query-svc  │───▶│  Redis Cache │──miss──▶│  (RDS)       │
│  (GET)      │    │  :8081      │    │  TTL: 10s    │         └──────────────┘
└─────────────┘    └─────────────┘    └──────────────┘
                         │
                         └──▶ X-Source: cache | db  (response header)
```

---

## Services

| Service | Port | Description |
|---|---|---|
| `event-svc` | 8080 | Accepts game events, publishes to Kafka |
| `stats-worker` | 8082 | Consumes Kafka, aggregates stats to Postgres |
| `query-svc` | 8081 | Serves fan reads via Redis cache + Postgres |

---

## Tech Stack

| Layer | Technology |
|---|---|
| Services | Go 1.24 |
| Message Queue | Apache Kafka (franz-go) |
| Cache | Redis 7 |
| Database | PostgreSQL 16 |
| Containerization | Docker + Docker Compose |
| Cloud Orchestration | AWS ECS Fargate + ALB |
| Infrastructure as Code | Terraform |
| Load Testing | Locust (Python) |

---

## Run Locally

**Prerequisites:** Docker Desktop, Go 1.24

```bash
# clone the repo
git clone https://github.com/Prannov/CS6650_Scalable_Distributed_Systems.git
cd CS6650_Scalable_Distributed_Systems/sportspulse

# start all 7 containers
docker compose -p sp-kafka -f docker-compose.yml up -d --build

# create Kafka topic (first time only)
docker exec sp-kafka-kafka-1 \
  kafka-topics --bootstrap-server localhost:9092 \
  --create --if-not-exists --topic game-events \
  --partitions 3 --replication-factor 1

# verify all services are healthy
curl http://localhost:8080/health   # event-svc
curl http://localhost:8081/health   # query-svc
```

---

## Try It

```bash
# send a game event
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"event_id":"e1","player_id":"p1","team_id":"lakers","event_type":"shot","value":3.0}'

# query player stats (first call: db, second call: cache)
curl http://localhost:8081/stats?player_id=p1

# leaderboard
curl http://localhost:8081/leaderboard
```

**Players seeded:** LeBron James (p1), Stephen Curry (p2), Kevin Durant (p3), Giannis A. (p4), Nikola Jokic (p5)

**Event types:** `shot` (2 or 3 pts), `assist` (1), `rebound` (1)

---

## Run Experiments - "CODE!"

```bash
pip install locust
cd load-tests
mkdir -p results

# Experiment 1 — Kafka vs Direct DB Writes
locust -f experiment1_kafka_vs_direct.py --host http://localhost:8080 \
  --users 200 --spawn-rate 20 --run-time 60s --headless \
  --csv results/exp1_kafka_200

# Experiment 2 — Redis Cache vs No Cache
locust -f experiment2_cache_vs_db.py --host http://localhost:8081 \
  --users 500 --spawn-rate 50 --run-time 60s --headless \
  --csv results/exp2_cache_500

# Experiment 3 — ECS Horizontal Scaling (requires AWS deployment)
locust -f experiment3_scaling.py \
  --host http://sportspulse-alb-33524114.us-east-1.elb.amazonaws.com:8081 \
  --users 300 --spawn-rate 30 --run-time 60s --headless \
  --csv results/exp3_tasks_1
```

---

## AWS Deployment

```bash
cd infra/terraform
terraform init
terraform apply -auto-approve

# scale query-svc for Experiment 3
terraform apply -var="query_svc_count=4" -auto-approve
```

**Live endpoints (while AWS session is active):**
- `http://sportspulse-alb-33524114.us-east-1.elb.amazonaws.com/health` — event-svc
- `http://sportspulse-alb-33524114.us-east-1.elb.amazonaws.com:8081/health` — query-svc

---

## Experiment Results Summary

### Experiment 1 — Kafka vs Direct DB Writes

| Users | Kafka req/s | Direct req/s | Kafka max | Direct max |
|---|---|---|---|---|
| 50 | 354 | 354 | 100ms | 58ms |
| 200 | 1,071 | 1,061 | 450ms | 1,100ms |
| 500 | 1,065 | 1,014 | 1,200ms | 2,500ms |

### Experiment 2 — Redis Cache vs No Cache

| Users | Cache p95 | No Cache p95 | Cache req/s | No Cache req/s |
|---|---|---|---|---|
| 100 | 31ms | 32ms | 1,012 | 1,007 |
| 500 | 310ms | 330ms | 1,250 | 1,219 |
| 1000 | 670ms | 750ms | 1,177 | 1,166 |

### Experiment 3 — ECS Horizontal Scaling

| Tasks | req/s | p50 | p95 | Error Rate |
|---|---|---|---|---|
| 1 | 111 | 2,100ms | 5,500ms | 45% |
| 2 | 255 | 600ms | 2,700ms | 18% |
| 4 | 513 | 45ms | 2,400ms | 2.4% |
| 8 | 797 | 46ms | 1,500ms | 0% |

---

## Repository Structure

```
sportspulse/
├── event-svc/          # Go — Kafka producer, game event ingestion
├── stats-worker/       # Go — Kafka consumer, stats aggregation
├── query-svc/          # Go — Redis + Postgres read service
├── event-svc-direct/   # Go — direct DB write variant (Experiment 1)
├── query-svc-nocache/  # Go — no Redis variant (Experiment 2)
├── infra/
│   ├── schema.sql      # Postgres schema + seed data
│   └── terraform/      # AWS ECS, RDS, ALB, ECR, VPC
├── load-tests/         # Locust scripts for all 3 experiments
│   └── results/        # CSV output from all experiment runs
├── docker-compose.yml          # Full Kafka stack
└── docker-compose.direct.yml   # Direct DB variant stack
```
