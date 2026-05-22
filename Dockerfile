FROM --platform=$BUILDPLATFORM node:24-alpine AS web-builder
WORKDIR /workspace/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend-builder
WORKDIR /workspace/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
RUN target_os="${TARGETOS:-$(go env GOOS)}" && \
    target_arch="${TARGETARCH:-$(go env GOARCH)}" && \
    echo "Building backend for ${target_os}/${target_arch}" && \
    CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
      go build \
      -ldflags="-s -w -X imagestudio/internal/buildinfo.Version=${VERSION} -X imagestudio/internal/buildinfo.Commit=${COMMIT} -X imagestudio/internal/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/image-studio .
RUN target_os="${TARGETOS:-$(go env GOOS)}" && \
    target_arch="${TARGETARCH:-$(go env GOARCH)}" && \
    echo "Building updater for ${target_os}/${target_arch}" && \
    CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
      go build \
      -ldflags="-s -w" \
      -o /out/image-studio-updater ./cmd/updater

FROM --platform=$BUILDPLATFORM alpine:3.22 AS runtime-assets
RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose tzdata && update-ca-certificates

FROM alpine:3.22

WORKDIR /app

COPY --from=runtime-assets /etc/ssl/certs /etc/ssl/certs
COPY --from=runtime-assets /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=runtime-assets /usr/bin/docker /usr/bin/docker
COPY --from=runtime-assets /usr/libexec/docker /usr/libexec/docker

COPY --from=backend-builder /out/image-studio /app/image-studio
COPY --from=backend-builder /out/image-studio-updater /app/image-studio-updater
COPY backend/internal/config/config.defaults.toml /app/data/config.example.toml
COPY --from=web-builder /workspace/web/dist /app/static

EXPOSE 7000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:7000/health >/dev/null || exit 1

CMD ["./image-studio"]
