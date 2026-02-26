# ============================================================
# Stage 1: Build the millibee binary
# ============================================================
FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN make build

# ============================================================
# Stage 2: Minimal runtime image
# ============================================================
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata curl yt-dlp git

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:18790/health || exit 1

# Copy binary and entrypoint
COPY --from=builder /src/build/millibee /usr/local/bin/millibee
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Create non-root user and group
RUN addgroup -g 1000 millibee && \
    adduser -D -u 1000 -G millibee millibee

# Switch to non-root user
USER millibee

# Run onboard to create initial directories, config, and skills
RUN /usr/local/bin/millibee onboard

# Preserve built-in skills separately so they survive volume mounts.
# The entrypoint script syncs these into the workspace on startup.
RUN cp -r /home/millibee/.millibee/workspace/skills /home/millibee/.millibee/_builtin_skills

WORKDIR /home/millibee

ENTRYPOINT ["entrypoint.sh"]
CMD ["gateway"]
