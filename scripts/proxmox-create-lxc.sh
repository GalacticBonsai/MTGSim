#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# Proxmox LXC creation for MTGSim
# ─────────────────────────────────────────────────────────────
# Run this *on the Proxmox host* (your tinybox) as root or via sudo.
#
# What it does:
#   1. Downloads an Ubuntu LXC template (26.04, fallback 24.04).
#   2. Creates an unprivileged container (ID 200, adjust below).
#   3. Enables nesting so Docker can run inside the container.
#   4. Starts the container, installs Docker + Compose plugin.
#   5. Clones the MTGSim repo and generates secrets.
#   6. Prints the container IP and next steps.
# ─────────────────────────────────────────────────────────────

CT_ID="${CT_ID:-200}"                    # LXC ID (change if 200 is in use)
CT_HOSTNAME="${CT_HOSTNAME:-mtgsim}"
CT_MEMORY="${CT_MEMORY:-4096}"           # MB
CT_SWAP="${CT_SWAP:-1024}"              # MB
CT_CORES="${CT_CORES:-2}"
CT_DISK_SIZE="${CT_DISK_SIZE:-12}"      # GB
CT_STORAGE="${CT_STORAGE:-local-lvm}"   # Proxmox storage pool
CT_BRIDGE="${CT_BRIDGE:-vmbr0}"
CT_GW="${CT_GW:-192.168.1.1}"           # Your gateway — CHANGE ME
CT_IP="${CT_IP:-dhcp}"                  # "dhcp" or "192.168.1.X/24"

REPO_URL="${REPO_URL:-https://github.com/galacticbonsai/mtgsim.git}"
MTGSIM_DIR="/opt/mtgsim"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
err()   { echo -e "[✗] $*" >&2; exit 1; }

# ── Sanity checks ────────────────────────────────────────────
command -v pveam &>/dev/null || err "This script must be run on a Proxmox host."
command -v pct   &>/dev/null || err "pct not found — are you on Proxmox?"
[ "$EUID" -eq 0 ]              || err "Please run as root."

if pct status "$CT_ID" &>/dev/null; then
    err "CT $CT_ID already exists. Set CT_ID=<different-id> or remove it first."
fi

# ── Step 1: Download template ────────────────────────────────
# Try Ubuntu 26.04 first (current as of mid-2026), fall back to 24.04.
TEMPLATE=""
for try in "${CT_TEMPLATE:-}" \
           "ubuntu-26.04-standard_26.04-1_amd64.tar.zst" \
           "ubuntu-26.04-standard_26.04-2_amd64.tar.zst" \
           "ubuntu-24.04-standard_24.04-2_amd64.tar.zst"; do
    [ -z "$try" ] && continue
    path="/var/lib/vz/template/cache/$try"
    if [ -f "$path" ]; then
        TEMPLATE="$try"
        TEMPLATE_PATH="$path"
        info "Using cached template: $TEMPLATE"
        break
    fi
done

if [ -z "$TEMPLATE" ]; then
    # None cached — try downloading 26.04, fall back to 24.04
    info "No cached template found. Updating template list…"
    pveam update
    for try in "ubuntu-26.04-standard_26.04-1_amd64.tar.zst" \
               "ubuntu-26.04-standard_26.04-2_amd64.tar.zst" \
               "ubuntu-24.04-standard_24.04-2_amd64.tar.zst"; do
        info "Attempting download: $try"
        if pveam download "$CT_STORAGE" "$try" 2>/dev/null; then
            TEMPLATE="$try"
            TEMPLATE_PATH="/var/lib/vz/template/cache/$try"
            info "Downloaded: $TEMPLATE"
            break
        fi
        warn "Not available: $try"
    done
fi

[ -z "$TEMPLATE" ] && err "Could not find or download any Ubuntu template."

# ── Step 2: Create the container ─────────────────────────────
info "Creating LXC $CT_ID ($CT_HOSTNAME)…"
pct create "$CT_ID" "$TEMPLATE_PATH" \
    --hostname "$CT_HOSTNAME" \
    --memory "$CT_MEMORY" \
    --swap "$CT_SWAP" \
    --cores "$CT_CORES" \
    --rootfs "$CT_STORAGE:$CT_DISK_SIZE" \
    --net0 name=eth0,bridge="$CT_BRIDGE",ip="$CT_IP",gw="$CT_GW" \
    --unprivileged 1 \
    --features nesting=1,fuse=1 \
    --ostype ubuntu

# ── Step 3: Start and configure ──────────────────────────────
info "Starting container…"
pct start "$CT_ID"
sleep 3

info "Waiting for network…"
for i in $(seq 1 30); do
    CT_IP_ADDR=$(pct exec "$CT_ID" -- hostname -I 2>/dev/null | awk '{print $1}')
    [ -n "$CT_IP_ADDR" ] && break
    sleep 2
done
[ -z "$CT_IP_ADDR" ] && CT_IP_ADDR="<check with: pct enter $CT_ID>"

info "Installing Docker + Compose…"
pct exec "$CT_ID" -- bash -c '
    set -e
    apt-get update -qq
    apt-get install -y -qq ca-certificates curl
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    # Detect Ubuntu codename for Docker repo (noble=24.04, plucky=26.04)
    UBUNTU_CODENAME=$(lsb_release -cs 2>/dev/null || . /etc/os-release && echo "$VERSION_CODENAME")
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $UBUNTU_CODENAME stable" \
        > /etc/apt/sources.list.d/docker.list
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    systemctl enable docker
    usermod -aG docker ubuntu
'

# ── Step 4: Clone repo and generate secrets ──────────────────
info "Cloning MTGSim repo into $MTGSIM_DIR…"
pct exec "$CT_ID" -- bash -c "
    set -e
    apt-get install -y -qq git
    git clone '$REPO_URL' '$MTGSIM_DIR'
    cd '$MTGSIM_DIR'
    POSTGRES_PASSWORD=\$(openssl rand -hex 32)
    MTGSIM_AUTH_TOKEN=\$(openssl rand -hex 32)
    cat > .env << EOF
POSTGRES_PASSWORD=\$POSTGRES_PASSWORD
MTGSIM_AUTH_TOKEN=\$MTGSIM_AUTH_TOKEN
EOF
    echo
    echo '=== SAVE THESE SECRETS ==='
    echo \"POSTGRES_PASSWORD=\$POSTGRES_PASSWORD\"
    echo \"MTGSIM_AUTH_TOKEN=\$MTGSIM_AUTH_TOKEN\"
    echo '=========================='
"

# ── Step 5: Print summary ────────────────────────────────────
echo
info "────────────────────────────────────────────────"
info "LXC $CT_ID ($CT_HOSTNAME) is ready!"
info "────────────────────────────────────────────────"
echo
info "Connect to the container:"
echo "    pct enter $CT_ID"
echo "    ssh ubuntu@$CT_IP_ADDR"
echo
info "Start MTGSim (from inside the container):"
echo "    cd $MTGSIM_DIR"
echo "    docker compose up -d"
echo
info "Download card database first (one-time):"
echo "    docker compose run --rm api mtgsim-edh -log=META 2>&1 | head -5"
echo
info "Container IP: $CT_IP_ADDR"
info "API will be at: https://api.h7lla.com"
echo
