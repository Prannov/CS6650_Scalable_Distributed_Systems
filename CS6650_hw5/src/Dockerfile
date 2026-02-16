FROM alpine:latest

WORKDIR /app

# Copy the pre-built binary
COPY server .

# Make it executable
RUN chmod +x ./server

EXPOSE 8080
CMD ["./server"]