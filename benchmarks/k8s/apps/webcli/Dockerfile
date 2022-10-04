FROM golang:1.17-alpine
WORKDIR /app
COPY webcli .
COPY ./make-app.sh ./make.sh

# Add bash to alpine
RUN apk add --no-cache bash

# Build app
RUN ["./make.sh", "--parallel"]

ENTRYPOINT ["/app/start.sh"]
