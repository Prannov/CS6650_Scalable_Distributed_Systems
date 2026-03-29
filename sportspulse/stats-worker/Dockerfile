FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o stats-worker .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/stats-worker .
EXPOSE 8082
CMD ["./stats-worker"]
