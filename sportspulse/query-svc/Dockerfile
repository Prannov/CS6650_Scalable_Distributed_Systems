FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o query-svc .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/query-svc .
COPY static/ ./static/
EXPOSE 8081
CMD ["./query-svc"]