#!/usr/bin/env bash
# One-shot VPS bootstrap for the AI content platform.
# Target: Ubuntu 22.04 or 24.04, fresh $10 VPS.
# Usage (as root):
#   export ADMIN_HOST=admin.example.com
#   export ADMIN_USER=ops
#   export ADMIN_PASSWORD_HASH="$(caddy hash-password --plaintext 'choose-a-strong-password')"
#   export ANTHROPIC_API_KEY=sk-ant-...
#   ./bootstrap.sh

set -euo pipefail

REQUIRED_ENVS=(ADMIN_HOST ADMIN_USER ADMIN_PASSWORD_HASH ANTHROPIC_API_KEY)
for v in "${REQUIRED_ENVS[@]}"; do
  if [[ -z "${!v:-}" ]]; then
    echo "error: $v is required" >&2
    exit 1
  fi
done

echo "==> Installing base packages"
apt-get update
apt-get install -y curl ca-certificates debian-keyring debian-archive-keyring apt-transport-https ufw

echo "==> Installing Caddy"
if ! command -v caddy >/dev/null 2>&1; then
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
  apt-get update
  apt-get install -y caddy
fi

echo "==> Creating platform user + directories"
id platform >/dev/null 2>&1 || useradd --system --home /opt/platform --shell /usr/sbin/nologin platform
mkdir -p /opt/platform/bin /opt/platform/data /opt/platform/dashboard /etc/platform /var/log/platform /var/log/caddy
chown -R platform:platform /opt/platform /var/log/platform

echo "==> Writing env file"
cat > /etc/platform/env <<EOF
HTTP_ADDR=127.0.0.1:8080
DATA_DIR=/opt/platform/data
DB_PATH=/opt/platform/data/platform.db
SITES_DIR=/opt/platform/data/sites
DASHBOARD_DIR=/opt/platform/dashboard/dist
ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
CLAUDE_MODEL=${CLAUDE_MODEL:-claude-sonnet-4-20250514}
CLAUDE_MONTHLY_BUDGET_USD=${CLAUDE_MONTHLY_BUDGET_USD:-50}
PIPELINE_TICK=${PIPELINE_TICK:-30m}
OPTIMIZER_TICK=${OPTIMIZER_TICK:-168h}
MIN_ARTICLE_AGE_DAYS=${MIN_ARTICLE_AGE_DAYS:-60}
ENV=production
EOF
chmod 600 /etc/platform/env
chown root:root /etc/platform/env

echo "==> Installing Caddyfile"
export ADMIN_HOST ADMIN_USER ADMIN_PASSWORD_HASH
envsubst < deploy/Caddyfile > /etc/caddy/Caddyfile 2>/dev/null || cp deploy/Caddyfile /etc/caddy/Caddyfile
# Caddy v2 supports {$ENV} placeholders natively; no envsubst needed usually.

echo "==> Installing systemd unit"
cp deploy/platform.service /etc/systemd/system/platform.service
systemctl daemon-reload

echo "==> Enabling firewall (only 22, 80, 443)"
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
echo y | ufw enable || true

echo "==> Building server binary (expects Go 1.22+ installed)"
(cd "$(dirname "$0")/.." && go build -o /opt/platform/bin/server ./cmd/server)
chown platform:platform /opt/platform/bin/server

echo "==> Starting services"
systemctl enable --now platform
systemctl reload caddy || systemctl restart caddy

echo "==> Done. Admin: https://${ADMIN_HOST}/  (basic auth user: ${ADMIN_USER})"
echo "==> Next: configure DNS A record ${ADMIN_HOST} -> $(curl -s ifconfig.me)"
