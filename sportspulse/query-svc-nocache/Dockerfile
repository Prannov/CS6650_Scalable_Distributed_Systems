FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN go build -o query-svc-nocache .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/query-svc-nocache .
EXPOSE 8080
CMD ["./query-svc-nocache"]
