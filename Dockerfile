# -----------------------------------------
# Stage 1: Build Go binaries
# -----------------------------------------
FROM golang:latest AS builder

WORKDIR /src
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY ./src .

# Build ldappy CLI and API binaries
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ldappy ./cmd/cli
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ldappy_api ./cmd/api


# -----------------------------------------
# Stage 2: Runtime
# -----------------------------------------
FROM debian:trixie-slim AS final

LABEL org.opencontainers.image.title="openldap" \
      org.opencontainers.image.description="Unified API + CLI + slapd stack" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/kitechsoftware/openldap" \
      org.opencontainers.image.version="0.1.0"

ENV DEBIAN_FRONTEND=noninteractive

# Copy binaries from builder
COPY --from=builder /out/ldappy /usr/local/bin/ldappy
COPY --from=builder /out/ldappy_api /usr/local/bin/ldappy_api

RUN mkdir -p /opt/supervisor/
# Copy configs
COPY supervisord.template /opt/supervisor/supervisord.template
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/ldappy /usr/local/bin/ldappy_api /usr/local/bin/entrypoint.sh

# Install dependencies: OpenLDAP, supervisor, utils
RUN ldappy install --container && \
    apt-get install -y --no-install-recommends \
    supervisor tini gosu && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /var/log/supervisor /var/log/ldap /etc/ldappy && \
    chown -R openldap:openldap /var/log/ldap /etc/ldappy

# Expose ports
EXPOSE 389 636 8080

# Healthcheck for LDAP availability
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ldapwhoami -x -H ldap://localhost:389 || exit 1

# Use tini for signal forwarding and clean shutdown
ENTRYPOINT ["/usr/bin/tini", "--"]

# Entrypoint launches your init logic (starts slapd, then supervisor)
CMD ["/usr/local/bin/entrypoint.sh"]
