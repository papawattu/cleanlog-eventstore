# Use a multi-stage build to support multiple architectures
# Stage 1: Build stage
FROM golang:1.23.1 AS builder
LABEL org.opencontainers.image.source=https://github.com/papawattu/cleanlog-eventstore
LABEL org.opencontainers.image.description="A simple web app log cleaning house"
LABEL org.opencontainers.image.licenses=MIT

ARG USER=nouser

WORKDIR /app

RUN apt update && apt install -y librdkafka-dev

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN make build


# Stage 2: Final stage
FROM debian AS build-stage

ARG USER=nouser

WORKDIR /

COPY --from=builder /app/bin/eventstore /eventstore

RUN adduser $USER \
    && mkdir -p /etc/sudoers.d \
    && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
    && chmod 0440 /etc/sudoers.d/$USER

USER $USER

EXPOSE 3000

ENTRYPOINT ["/eventstore"]