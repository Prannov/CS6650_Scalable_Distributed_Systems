FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY store/ ./store/
COPY leaderless/ ./leaderless/

RUN CGO_ENABLED=0 go build -o /ll-node ./leaderless

FROM alpine:3.19
COPY --from=builder /ll-node /ll-node
ENTRYPOINT ["/ll-node"]