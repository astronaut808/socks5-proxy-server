ARG GOLANG_VERSION="1.27.3"

FROM golang:$GOLANG_VERSION-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags '-s -w' -o /out/socks5-proxy-server ./cmd/socks5-proxy-server

FROM gcr.io/distroless/static:nonroot
USER 65532:65532
COPY --from=builder /out/socks5-proxy-server /socks5-proxy-server
ENTRYPOINT ["/socks5-proxy-server"]
