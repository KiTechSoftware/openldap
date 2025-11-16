#!/bin/bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configurable Environment Variables
# ---------------------------------------------------------------------------
LDAP_DATA_DIR="${LDAP_DATA_DIR:-/var/lib/ldap}"
LDAP_CONFIG_DIR="${LDAP_CONFIG_DIR:-/etc/ldap/slapd.d}"
LDAP_LOG_DIR="${LDAP_LOG_DIR:-/var/log/ldap}"
LDAPPY_DIR="${LDAPPY_DIR:-/etc/ldappy}"
LDAP_RUN_DIR="${LDAP_RUN_DIR:-/run/slapd}"
LDAP_USER="${LDAP_USER:-openldap}"
LDAP_GROUP="${LDAP_GROUP:-openldap}"
LDAP_PORT="${LDAP_PORT:-389}"
LDAPS_PORT="${LDAPS_PORT:-636}"
LDAP_DEBUG_LEVEL="${LDAP_DEBUG_LEVEL:-0}"

LDAP_CERT_DIR="${LDAP_CERT_DIR:-/etc/ssl/ldap}"
LDAP_CERT_FILE="${LDAP_CERT_FILE:-$LDAP_CERT_DIR/server.crt}"
LDAP_KEY_FILE="${LDAP_KEY_FILE:-$LDAP_CERT_DIR/server.key}"

echo "[ldappy-entrypoint] 🚀 Starting OpenLDAP initialization..."

# ---------------------------------------------------------------------------
# LDAP User/Group Handling
# ---------------------------------------------------------------------------
# Check if the group exists; if not, create it
if ! getent group "$LDAP_GROUP" > /dev/null 2>&1; then
  echo "[ldappy-entrypoint] Creating group '$LDAP_GROUP'..."
  groupadd --system "$LDAP_GROUP"
fi

# Check if the user exists; if not, create it
if ! id "$LDAP_USER" > /dev/null 2>&1; then
  echo "[ldappy-entrypoint] Creating user '$LDAP_USER'..."
  useradd --system --no-create-home --shell /usr/sbin/nologin \
    --gid "$LDAP_GROUP" "$LDAP_USER"
fi

echo "[ldappy-entrypoint] Using LDAP user/group: $LDAP_USER:$LDAP_GROUP"

# ---------------------------------------------------------------------------
# Ensure directories exist and ownerships are correct
# ---------------------------------------------------------------------------
mkdir -p "$LDAP_DATA_DIR" "$LDAP_CONFIG_DIR" "$LDAP_LOG_DIR" "$LDAPPY_DIR" "$LDAP_CERT_DIR" "$LDAP_RUN_DIR"
chown -R "$LDAP_USER:$LDAP_GROUP" "$LDAP_DATA_DIR" "$LDAP_CONFIG_DIR" "$LDAP_LOG_DIR" "$LDAPPY_DIR" "$LDAP_RUN_DIR" 

# ---------------------------------------------------------------------------
# Generate self-signed certs if missing (for LDAPS)
# ---------------------------------------------------------------------------
if [ ! -f "$LDAP_CERT_FILE" ] || [ ! -f "$LDAP_KEY_FILE" ]; then
  echo "[ldappy-entrypoint] Generating self-signed TLS certificate for LDAPS..."
  openssl req -x509 -nodes -newkey rsa:2048 \
    -keyout "$LDAP_KEY_FILE" \
    -out "$LDAP_CERT_FILE" \
    -subj "/CN=localhost" \
    -days 365
  chmod 600 "$LDAP_KEY_FILE"
  chown "$LDAP_USER:$LDAP_GROUP" "$LDAP_CERT_FILE" "$LDAP_KEY_FILE"
fi

# ---------------------------------------------------------------------------
# Start slapd temporarily for ldappy init
# ---------------------------------------------------------------------------
echo "[ldappy-entrypoint] Starting temporary slapd..."
/usr/sbin/slapd -h "ldap://127.0.0.1:$LDAP_PORT ldapi:///" \
  -u "$LDAP_USER" -g "$LDAP_GROUP" -d 0 &

# ---------------------------------------------------------------------------
# Wait for slapd readiness
# ---------------------------------------------------------------------------
tries=0
until ldapwhoami -x -H "ldap://127.0.0.1:$LDAP_PORT" >/dev/null 2>&1; do
  tries=$((tries+1))
  if [ "$tries" -gt 15 ]; then
    echo "[ldappy-entrypoint] ❌ slapd failed to start for initialization." >&2
    pkill slapd || true
    exit 1
  fi
  echo "[ldappy-entrypoint] Waiting for slapd to become ready ($tries/15)..."
  sleep 1
done
echo "[ldappy-entrypoint] ✅ slapd is ready."

# ---------------------------------------------------------------------------
# Run ldappy initialization
# ---------------------------------------------------------------------------
if ! /usr/local/bin/ldappy init --force; then
  echo "[ldappy-entrypoint] ❌ ldappy initialization failed!" >&2
  pkill slapd || true
  exit 1
fi

# ---------------------------------------------------------------------------
# Stop temporary slapd
# ---------------------------------------------------------------------------
echo "[ldappy-entrypoint] Stopping temporary slapd..."
pkill slapd || true
sleep 1

# ---------------------------------------------------------------------------
# Fix ownership again (in case ldappy changed permissions)
# ---------------------------------------------------------------------------
chown -R "$LDAP_USER:$LDAP_GROUP" "$LDAP_DATA_DIR" "$LDAP_CONFIG_DIR" "$LDAP_LOG_DIR" "$LDAPPY_DIR"

# ---------------------------------------------------------------------------
# Render dynamic Supervisor configuration
# ---------------------------------------------------------------------------
SUPERVISOR_TEMPLATE="/opt/supervisor/supervisord.template"
SUPERVISOR_CONF="/etc/supervisor/conf.d/supervisord.conf"

# If config already exists, skip generation and continue
if [ -f "$SUPERVISOR_CONF" ]; then
  echo "[ldappy-entrypoint] Supervisor config already exists, skipping generation."
else
  # If config does NOT exist, check for template and generate
  if [ -f "$SUPERVISOR_TEMPLATE" ]; then
    echo "[ldappy-entrypoint] Generating Supervisor config for user '$LDAP_USER'..."
    sed "s|__LDAP_USER__|$LDAP_USER|g" "$SUPERVISOR_TEMPLATE" > "$SUPERVISOR_CONF"
  else
    echo "[ldappy-entrypoint] ⚠️ Supervisor config template not found: $SUPERVISOR_TEMPLATE"
  fi
fi

# ---------------------------------------------------------------------------
# Trap for clean shutdown (Docker stop)
# ---------------------------------------------------------------------------
trap 'echo "[ldappy-entrypoint] 🔻 Shutting down slapd..."; pkill slapd || true; exit 0' SIGTERM SIGINT

# ---------------------------------------------------------------------------
# Tail logs in background so docker logs shows live output
# ---------------------------------------------------------------------------
touch "$LDAP_LOG_DIR/slapd.log"
chown "$LDAP_USER:$LDAP_GROUP" "$LDAP_LOG_DIR/slapd.log"
tail -F "$LDAP_LOG_DIR/slapd.log" &
TAIL_PID=$!

# ---------------------------------------------------------------------------
# Final foreground start (Docker PID 1)
# ---------------------------------------------------------------------------
echo "[ldappy-entrypoint] 🚀 Starting production slapd..."
# Drop to openldap user for Supervisor
# if [ "$(id -u)" = 0 ]; then
#   exec gosu "$LDAP_USER" /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf
# else
#   exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf
# fi

exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf