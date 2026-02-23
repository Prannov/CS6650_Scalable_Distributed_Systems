FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY server .
EXPOSE 8080
CMD ["./server"]
