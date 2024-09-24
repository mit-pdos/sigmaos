FROM golang:1.22-alpine
WORKDIR /app

# Add bash to alpine
RUN apk add --no-cache bash curl

# Copy the test file
COPY ./tcp_test.go .

# Init module and download dependencies
RUN go mod init netperf && go mod tidy && go mod download
