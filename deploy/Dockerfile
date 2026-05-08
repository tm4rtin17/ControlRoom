# syntax=docker/dockerfile:1.6
#
# Multi-stage build for ControlRoom:
#   1. node-builder  — `vite build` → web/dist/
#   2. go-builder    — `go build` with embed.FS picking up web/dist
#   3. runtime       — distroless static, nonroot user, ~20 MB final image

# ---------- 1. frontend ----------
FROM node:20-alpine AS web-builder

WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY web/ ./
RUN npm run build

# ---------- 2. backend ----------
FROM golang:1.23-alpine AS go-builder

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

RUN apk add --no-cache ca-certificates git

# Pre-warm module cache before copying full source.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/web/dist ./web/dist

# Pre-create the data dir so we can COPY it into the runtime with the right
# ownership (distroless has no shell to run mkdir/chown at runtime).
RUN mkdir -p /out/data

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
      -X github.com/tm4rtin17/controlroom/internal/buildinfo.Version=${VERSION} \
      -X github.com/tm4rtin17/controlroom/internal/buildinfo.Commit=${COMMIT} \
      -X github.com/tm4rtin17/controlroom/internal/buildinfo.Date=${DATE}" \
    -o /out/controlroom ./cmd/controlroom

# ---------- 3. runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=go-builder /out/controlroom /app/controlroom
COPY --from=go-builder --chown=65532:65532 /out/data /data

ENV CR_DATA_DIR=/data \
    CR_ADDR=:8443

VOLUME ["/data"]
EXPOSE 8443
USER 65532:65532

ENTRYPOINT ["/app/controlroom"]
