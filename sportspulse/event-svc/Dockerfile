FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o event-svc .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/event-svc .
EXPOSE 8080
CMD ["./event-svc"]