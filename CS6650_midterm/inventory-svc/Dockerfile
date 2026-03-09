FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN go build -o inventory-svc .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/inventory-svc .
EXPOSE 8081
CMD ["./inventory-svc"]