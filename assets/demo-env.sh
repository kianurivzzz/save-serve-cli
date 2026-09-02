#!/bin/sh
set -e
home=$(mktemp -d)
mkdir -p "$home/.config/sv" "$home/.ssh"
touch "$home/.ssh/id_ed25519"
cat > "$home/.config/sv/hosts.yaml" <<'YAML'
version: 1
defaults:
  user: root
  port: 22
  key: ~/.ssh/id_ed25519
groups:
  - name: home
    color: green
  - name: work
    color: orange
hosts:
  - name: coolify
    host: 5.75.128.14
    auth: key
    group: home
    tags: [docker, prod]
  - name: pi
    host: 192.168.1.20
    user: pi
    auth: agent
    group: home
  - name: bastion
    host: bastion.example.com
    auth: key
    group: work
  - name: db-primary
    host: 10.1.0.5
    user: postgres
    port: 2222
    auth: key
    group: work
    jump: bastion
    tags: [prod, db]
  - name: CashCow
    host: cashcow.example.com
    auth: password
    group: work
YAML
ago() { date -u -v-"$1" +%FT%TZ 2>/dev/null || date -u -d "-$2" +%FT%TZ; }
cat > "$home/.config/sv/state.json" <<JSON
{"last_used": {"coolify": "$(ago 2H '2 hours')", "db-primary": "$(ago 1d '1 day')", "cashcow": "$(ago 3d '3 days')"}}
JSON
echo "export HOME=$home"
