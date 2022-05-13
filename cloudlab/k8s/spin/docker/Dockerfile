FROM golang:1.17-alpine
WORKDIR /app
COPY . .

# Add bash to alpine
RUN apk add --no-cache bash

## Make the out directory
#RUN ["mkdir", "-p", "/app/out"]

# XXX Should I try to get rid of this?
# Build app
RUN ["./make.sh", "-norace"]

ENTRYPOINT ["/app/start.sh"]
