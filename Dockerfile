# DesignHub image: the Go server with the built frontend embedded, plus the
# PostgreSQL client tools used for backups. Builder stages run on the build
# machine's platform and cross-compile, so multi-architecture builds are fast.

FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS server
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/embed.go
COPY --from=web /src/web/dist/ web/dist/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -mod=vendor -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" -o /out/designhub ./cmd/designhub

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata postgresql17-client \
    && addgroup -S -g 65532 designhub \
    && adduser -S -D -H -u 65532 -G designhub designhub \
    && mkdir -p /data/repo /data/files /backups /secrets \
    && chown -R 65532:65532 /data /backups
COPY --from=server /out/designhub /usr/local/bin/designhub
USER 65532:65532
ENV DESIGNHUB_ADDR=:8080 \
    DESIGNHUB_REPO=/data/repo \
    DESIGNHUB_FILES=/data/files \
    DESIGNHUB_BACKUPS=/backups \
    DESIGNHUB_SECRETS=/secrets
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s CMD ["designhub", "healthcheck"]
ENTRYPOINT ["designhub"]
CMD ["serve"]
LABEL org.opencontainers.image.title="DesignHub" \
      org.opencontainers.image.source="https://github.com/Shwhat/DesignHub"
