FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o envoy ./cmd/envoy

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /build/envoy /usr/local/bin/envoy
COPY --from=builder /build/configs/envoy.yaml /etc/envoy/envoy.yaml

EXPOSE 7470

CMD ["envoy", "--config", "/etc/envoy/envoy.yaml"]
