# syntax=docker/dockerfile:1-experimental

FROM alpine as base
ARG APP_NAME
WORKDIR /app
COPY ${APP_NAME} .

RUN mkdir /app/bin

RUN apk add --no-cache libseccomp gcompat libpthread-stubs musl-dev bash

# Copy binaries from builder
COPY --from=arielszekely/appbuilder /app/bin/* /app/bin/

ENTRYPOINT ["/app/start.sh"]

# XXX

#FROM golang:1.19-alpine
#ARG APP_NAME
#WORKDIR /app
#COPY ${APP_NAME} .
#COPY ./make-app.sh ./make.sh
#
## Add bash to alpine
#RUN apk add --no-cache bash
#
## Build app
#RUN ["./make.sh", "--parallel"]
#
#ENTRYPOINT ["/app/start.sh"]
