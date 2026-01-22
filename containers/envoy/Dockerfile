# Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o envoy ./cmd/envoy

# Stage 2: Runtime on shared base
ARG BASE_TAG=latest
FROM protectorate/base:${BASE_TAG}

COPY --from=builder /build/envoy /usr/local/bin/envoy
COPY --from=builder /build/configs/envoy.yaml /etc/envoy/envoy.yaml
COPY containers/envoy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 7470

ENTRYPOINT ["/entrypoint.sh"]
