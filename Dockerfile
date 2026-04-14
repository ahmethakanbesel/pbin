# ---- Build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 go build -mod=vendor \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /pbin ./cmd/pbin

# Create data directory to copy into distroless (which has no shell/mkdir)
RUN mkdir -p /data

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12

COPY --from=builder /pbin /pbin

# Data directory for SQLite DB and uploads.
# Distroless has no shell, so we copy an empty dir from the builder.
COPY --from=builder --chown=nonroot:nonroot /data /data

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/pbin"]
CMD ["--config", ""]
