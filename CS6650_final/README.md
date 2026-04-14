# Album Store — CS 6650 ChaosArena v1

## Stack
- **API**: Go + chi router
- **DB**: PostgreSQL (albums + photo metadata)
- **Cache / seq counter**: Redis INCR (atomic per-album sequence)
- **File storage**: AWS S3 (presigned URLs returned to ChaosArena)
- **Queue**: AWS SQS (decouples upload acceptance from completion marking)
- **Deploy**: AWS ECS Fargate (API + Worker as separate services)

## Local Development

### Prerequisites
- Docker + Docker Compose
- `uuidgen` (macOS built-in; Linux: `apt install uuid-runtime`)
- `jq`

### Run locally
```bash
docker compose up --build
```

### Smoke test
```bash
chmod +x scripts/smoke-test.sh
./scripts/smoke-test.sh
```

## Architecture

```
Client
  │
  ▼
ALB (port 80)
  │
  ▼
ECS Service: API (2+ tasks, port 8080)
  ├── PUT/GET /albums        → PostgreSQL (RDS)
  ├── GET /albums            → PostgreSQL (RDS)
  ├── POST /albums/*/photos  → Redis INCR (seq) → S3 upload → SQS publish → 202
  ├── GET /albums/*/photos/* → PostgreSQL
  └── DELETE /albums/*/photos/* → S3 delete → PostgreSQL delete
  
ECS Service: Worker (2+ tasks)
  └── SQS poll → mark photo completed in PostgreSQL
```

## Key Design Decisions

### Atomic `seq` via Redis INCR
Each album has a Redis key `seq:<album_id>`. `INCR` is atomic — no two
concurrent uploads to the same album will ever get the same seq number.

### S3 upload happens in the POST handler
The photo is uploaded to S3 synchronously in the POST handler before the 202
is returned. The SQS message carries the already-generated presigned URL.
The worker only needs to flip the DB status to `completed` — no S3 work.
This makes the worker very fast and crash-safe: if the worker dies, the
message returns to SQS after the visibility timeout and another worker picks
it up. The photo is already safely in S3.

### Why not upload from the worker?
If we put the raw bytes on SQS (25 KB limit) or stored them in Redis and the
worker crashed before uploading, we'd lose the photo. Uploading from the
handler ensures durability before we acknowledge the client.

## AWS Deployment

See `terraform/` directory for full IaC.

Environment variables required per ECS task:
```
DATABASE_URL      postgres://...
REDIS_ADDR        elasticache-endpoint:6379
SQS_QUEUE_URL     https://sqs...
S3_BUCKET         your-bucket
AWS_REGION        us-east-1
```

IAM task role needs:
- `s3:PutObject`, `s3:DeleteObject`, `s3:GetObject` on your bucket
- `sqs:SendMessage`, `sqs:ReceiveMessage`, `sqs:DeleteMessage` on your queue
