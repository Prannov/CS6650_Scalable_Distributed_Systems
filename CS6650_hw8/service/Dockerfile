FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# cross-compile for Linux AMD64 (required from ARM Mac)
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o cart-service .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/cart-service .
EXPOSE 8080
CMD ["./cart-service"]