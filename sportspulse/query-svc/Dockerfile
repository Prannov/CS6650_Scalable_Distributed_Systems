FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o query-svc .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/query-svc .
EXPOSE 8081
CMD ["./query-svc"]
