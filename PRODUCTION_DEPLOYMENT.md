# MTGSim Production Deployment

## Architecture overview

```
GitHub Pages (frontend)
      │  fetch("https://api.h7lla.com/api/...")
      ▼
 Cloudflare (DNS / DDoS / Turnstile)
      │
      ▼
  Caddy (TLS termination, rate limiting)
      │
      ▼
 MTGSim API (job creation, reads)
      │           │
      ▼           ▼
 PostgreSQL   Runner workers
 (private)    (poll jobs table)
```

- **PostgreSQL** uses `expose: 5432` — reachable only inside the Docker network.
- **Caddy** is the only internet-facing service (ports 80/443).
- **API** handles browser requests; creates `simulation_jobs` on `POST /api/run-games`.
- **Runner workers** poll the jobs table, run simulations, and write results.

---

## Prerequisites (Proxmox)

1. **Create an LXC container or VM** (Debian/Ubuntu recommended).
   - Minimum 4 vCPU, 8 GB RAM, 20 GB disk.
   - Install Docker + Compose plugin:
     ```bash
     apt update && apt install -y docker.io docker-compose-v2
     ```

2. **DNS** — On Namecheap, add an A record:
   ```
   api  A  <your-proxmox-ip>
   ```
   Remove the existing URL redirect (`h7lla.com → www.h7lla.com`) or keep it — Caddy handles root traffic.

3. **Firewall** — Only open ports 80 and 443 (or tunnel through Cloudflare).

---

## Step 1: Clone and configure

```bash
git clone https://github.com/galacticbonsai/mtgsim.git /opt/mtgsim
cd /opt/mtgsim

# Create .env with secrets
cat > .env << 'EOF'
POSTGRES_PASSWORD=$(openssl rand -hex 32)
MTGSIM_AUTH_TOKEN=$(openssl rand -hex 32)
EOF

# Source the env file
set -a; source .env; set +a
```

**What these do:**

| Variable | Purpose |
|---|---|
| `POSTGRES_PASSWORD` | PostgreSQL password (never expose to the internet) |
| `MTGSIM_AUTH_TOKEN` | Bearer token required for all write API endpoints |

---

## Step 2: Download the Scryfall card database

```bash
# The API container needs the card cache; run once:
docker compose run --rm api mtgsim-edh -log=META 2>&1 | head -20
# This downloads ~70 MB card database to .cache/
```

---

## Step 3: Configure Caddy

Edit `Caddyfile`:

```
api.h7lla.com {
    rate_limit {
        zone dynamic {
            key {remote_host}
            events 100
            window 1m
        }
    }
    reverse_proxy api:8080
}
```

Already set to `api.h7lla.com`. Change `Caddyfile` if you use a different domain.

---

## Step 4: Start the stack

```bash
docker compose up -d
```

Check logs:

```bash
docker compose logs -f api      # API server
docker compose logs -f runner   # Worker (processes simulation jobs)
docker compose logs -f caddy    # TLS / reverse proxy
```

---

## Step 5: Verify

```bash
# Health check (public, no auth needed)
curl https://api.h7lla.com/api/health

# Run a simulation (requires auth token)
curl -X POST https://api.h7lla.com/api/run-games?count=50 \
  -H "Authorization: Bearer $MTGSIM_AUTH_TOKEN"

# Check status
curl https://api.h7lla.com/api/game-status

# View results (public)
curl https://api.h7lla.com/api/edh-results
```

---

## Step 6: Connect GitHub Pages frontend

In your GitHub Pages site, point all API calls to your domain:

```javascript
const API = "https://api.h7lla.com";

async function fetchResults() {
  const res = await fetch(`${API}/api/edh-results`);
  return res.json();
}

async function runSimulation(count) {
  const token = prompt("Enter API token"); // or load from secure storage
  const res = await fetch(`${API}/api/run-games?count=${count}`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
  });
  return res.json();
}
```

---

## Scaling runners

If one worker isn't enough, scale the runner service:

```bash
docker compose up -d --scale runner=3
```

Each worker runs in its own container and claims jobs atomically via
`UPDATE ... FOR UPDATE SKIP LOCKED`. No duplicate work.

---

## Updating

```bash
cd /opt/mtgsim
git pull
docker compose build --no-cache api
docker compose up -d
```

---

## API reference

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/health` | No | Health check |
| GET | `/api/results` | No | 1v1 deck win/loss aggregates |
| GET | `/api/edh-results` | No | EDH per-deck stats |
| GET | `/api/edh-games` | No | Recent EDH pods |
| GET | `/api/game-status` | No | Queue status (pending/running jobs) |
| GET | `/api/decks` | No | Uploaded deck list |
| GET | `/api/card-library` | No | Global card stats |
| GET | `/api/card-search` | No | Search cards |
| POST | `/api/run-games` | Yes | Enqueue simulation job |
| POST | `/api/upload-deck` | Yes | Upload a deck file |
| DELETE | `/api/uploaded-decks` | Yes | Remove an uploaded deck |
| POST | `/api/save-snapshot` | Yes | Save meta snapshot |
| POST | `/api/reset-card-library` | Yes | Reset card stats |
| POST | `/api/reset-game-logs` | Yes | Clear game logs |

---

## Security checklist

- [ ] `POSTGRES_PASSWORD` and `MTGSIM_AUTH_TOKEN` are **not** the defaults.
- [ ] Port 5432 is **not** open on the host firewall.
- [ ] Only ports 80/443 are exposed on the host.
- [ ] Cloudflare proxying is enabled (orange cloud) for DDoS protection.
- [ ] Caddy is configured with rate limiting.
- [ ] Frontend never hard-codes the auth token.

---

## Troubleshooting

```bash
# See all logs
docker compose logs -f

# Reset database
docker compose down -v && docker compose up -d

# Rebuild from scratch
docker compose build --no-cache
docker compose up -d

# Run a single pod manually (for testing)
docker compose run --rm runner mtgsim-edh -pod=4 -games=1
```
