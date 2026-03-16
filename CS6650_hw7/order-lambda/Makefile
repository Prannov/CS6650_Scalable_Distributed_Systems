# order-lambda/Makefile  (Lambda needs provided.al2 bootstrap binary)
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bootstrap .
	zip function.zip bootstrap