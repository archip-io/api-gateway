FROM golang:alpine AS builder
WORKDIR /build
COPY . .


RUN go mod tidy && \
    go build -o proxy ./cmd/api-gateway/main.go

FROM alpine
WORKDIR /build

COPY --from=builder /build/proxy /build/proxy
COPY --from=builder /build/configs /build/configs

ENV PORT=1234

CMD ./proxy --port $PORT