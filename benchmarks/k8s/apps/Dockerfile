FROM golang:1.19-alpine
ARG APP_NAME
WORKDIR /app
COPY ${APP_NAME} .
COPY ./make-app.sh ./make.sh

# Add bash to alpine
RUN apk add --no-cache bash

# Build app
RUN ["./make.sh", "--parallel"]

ENTRYPOINT ["/app/start.sh"]
