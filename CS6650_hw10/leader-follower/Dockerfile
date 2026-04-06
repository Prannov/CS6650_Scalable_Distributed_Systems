FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module files first for better layer caching
COPY go.mod ./
RUN go mod download

# Copy source
COPY store/ ./store/
COPY leader-follower/ ./leader-follower/

# Build for linux/amd64 explicitly (ARM Mac → ECS/Docker)
RUN CGO_ENABLED=0 go build -o /lf-node ./leader-follower

FROM alpine:3.19
COPY --from=builder /lf-node /lf-node
ENTRYPOINT ["/lf-node"]