FROM golang:1.17.2 AS builder

RUN mkdir /app
WORKDIR /app
COPY . /app

RUN GOOS=linux GARCH=amd64 CGO_ENABLED=0 go build -o dotcom_monitor

###############################################

FROM alpine:3.14.0

LABEL MAINTAINER="Kevin van den Broek <info@kevinvandenbroek.nl>"

COPY --from=builder /app/dotcom_monitor /

ENTRYPOINT ["/dotcom_monitor"]
