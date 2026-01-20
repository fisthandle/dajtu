# Resource Limits Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Dodać jawne limity zasobów (50GB dysk, 10Mbps transfer, 2 CPU cores) w Docker i upewnić się że config aplikacji jest spójny.

**Architecture:** Limity zasobów na dwóch poziomach:
1. **Docker** - hard limits na CPU, memory, network (cgroups)
2. **Aplikacja** - soft limits na dysk z cleanup daemonem (już istnieje, tylko weryfikacja)

**Tech Stack:** Docker Compose v3.9+, Go config

---

## Task 1: Rozdziel docker-compose na local i prod

**Files:**
- Modify: `docker-compose.yml` (base - local dev)
- Create: `docker-compose.prod.yml` (production overrides)

**Step 1: Zmodyfikuj docker-compose.yml na wersję local**

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - DATA_DIR=/data
      - BASE_URL=http://localhost:8080
      - MAX_FILE_SIZE_MB=20
      - MAX_DISK_GB=10
      - CLEANUP_TARGET_GB=8
    volumes:
      - ./data:/data
    restart: unless-stopped
```

**Step 2: Utwórz docker-compose.prod.yml**

```yaml
# Production overrides
# Usage: docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

services:
  app:
    environment:
      - BASE_URL=https://dajtu.com
      - MAX_DISK_GB=50
      - CLEANUP_TARGET_GB=45
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 256M
```

**Step 3: Zwaliduj oba pliki**

```bash
# Local config
docker-compose config

# Production config (merged)
docker-compose -f docker-compose.yml -f docker-compose.prod.yml config
```

Expected: Valid YAML output bez błędów dla obu

**Step 4: Commit**

```bash
git add docker-compose.yml docker-compose.prod.yml
git commit -m "feat: split docker-compose into local and prod configs"
```

---

## Task 2: Dodaj limit bandwidth przez traffic control

**Files:**
- Create: `scripts/setup-network-limit.sh`
- Modify: `docker-compose.yml` (opcjonalnie)

**Kontekst:** Docker nie ma natywnego limitu bandwidth. Opcje:
1. `tc` (traffic control) na hoście - najprostsze
2. Wondershaper w kontenerze
3. Docker plugin (np. netshaper)

Wybieramy `tc` na hoście jako najprostsze rozwiązanie.

**Step 1: Utwórz skrypt setup-network-limit.sh**

```bash
#!/bin/bash
# Limit bandwidth for dajtu container to 10 Mbps
# Run as root on the host after container starts

CONTAINER_NAME="dajtu-app-1"
RATE="10mbit"
BURST="1mbit"

# Get container's veth interface
CONTAINER_PID=$(docker inspect -f '{{.State.Pid}}' "$CONTAINER_NAME" 2>/dev/null)

if [ -z "$CONTAINER_PID" ] || [ "$CONTAINER_PID" = "0" ]; then
    echo "Container $CONTAINER_NAME not running"
    exit 1
fi

# Find veth peer on host
VETH=$(ip link | grep -E "veth.*@if" | grep -oP 'veth[a-f0-9]+' | head -1)

if [ -z "$VETH" ]; then
    echo "Could not find veth interface for container"
    exit 1
fi

# Apply traffic control - limit egress (upload from container perspective)
tc qdisc del dev "$VETH" root 2>/dev/null
tc qdisc add dev "$VETH" root tbf rate "$RATE" burst "$BURST" latency 50ms

echo "Applied $RATE limit to $VETH (container: $CONTAINER_NAME)"
```

**Step 2: Make executable**

```bash
chmod +x scripts/setup-network-limit.sh
```

**Step 3: Test skrypt (opcjonalne jeśli masz działający kontener)**

```bash
sudo ./scripts/setup-network-limit.sh
```

**Step 4: Commit**

```bash
git add scripts/setup-network-limit.sh
git commit -m "feat: add network bandwidth limit script (10 Mbps)"
```

---

## Task 3: Zweryfikuj i udokumentuj limity dysku w config

**Files:**
- Modify: `internal/config/config.go` (dodaj komentarze/walidację)
- Modify: `README.md` lub utwórz `docs/DEPLOYMENT.md`

**Step 1: Dodaj walidację w config.go**

W `internal/config/config.go` po linii 26, dodaj walidację:

```go
func Load() *Config {
    cfg := &Config{
        Port:          getEnv("PORT", "8080"),
        DataDir:       getEnv("DATA_DIR", "./data"),
        MaxFileSizeMB: getEnvInt("MAX_FILE_SIZE_MB", 20),
        MaxDiskGB:     getEnvFloat("MAX_DISK_GB", 50.0),
        CleanupTarget: getEnvFloat("CLEANUP_TARGET_GB", 45.0),
        BaseURL:       getEnv("BASE_URL", ""),
    }

    // Validate: CleanupTarget must be less than MaxDiskGB
    if cfg.CleanupTarget >= cfg.MaxDiskGB {
        cfg.CleanupTarget = cfg.MaxDiskGB * 0.9 // 90% of max
    }

    return cfg
}
```

**Step 2: Dodaj test walidacji**

W `internal/config/config_test.go` dodaj:

```go
func TestLoad_CleanupTargetValidation(t *testing.T) {
    // Set invalid values: CleanupTarget >= MaxDiskGB
    t.Setenv("MAX_DISK_GB", "50")
    t.Setenv("CLEANUP_TARGET_GB", "60") // Invalid: greater than max

    cfg := Load()

    // Should auto-correct to 90% of MaxDiskGB
    expected := 50.0 * 0.9
    if cfg.CleanupTarget != expected {
        t.Errorf("CleanupTarget = %v, want %v (90%% of MaxDiskGB)", cfg.CleanupTarget, expected)
    }
}
```

**Step 3: Uruchom testy**

```bash
go test ./internal/config/... -v
```

Expected: All tests PASS

**Step 4: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add validation for CleanupTarget < MaxDiskGB"
```

---

## Task 4: Utwórz dokumentację deployment

**Files:**
- Create: `docs/DEPLOYMENT.md`

**Step 1: Utwórz docs/DEPLOYMENT.md**

```markdown
# Dajtu Deployment Guide

## Resource Limits

### Disk Storage
- **Max storage:** 50 GB (configurable via `MAX_DISK_GB`)
- **Cleanup trigger:** When usage exceeds `MAX_DISK_GB`
- **Cleanup target:** 45 GB (configurable via `CLEANUP_TARGET_GB`)
- **Strategy:** Deletes oldest images first (FIFO by `created_at`)
- **Interval:** Every 5 minutes

### CPU & Memory (Docker)
- **CPU limit:** 2 cores
- **Memory limit:** 2 GB
- **Memory reservation:** 256 MB

### Network Bandwidth
- **Limit:** 10 Mbps
- **Method:** Traffic control (`tc`) on host
- **Setup:** Run `sudo ./scripts/setup-network-limit.sh` after container starts

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP server port |
| `DATA_DIR` | ./data | Storage directory |
| `MAX_FILE_SIZE_MB` | 20 | Max single file upload size |
| `MAX_DISK_GB` | 50 | Disk usage limit (triggers cleanup) |
| `CLEANUP_TARGET_GB` | 45 | Target size after cleanup |
| `BASE_URL` | (empty) | Public URL for generated links |

## Quick Start

### Local Development
```bash
docker-compose up -d
docker-compose logs -f
```

### Production
```bash
# Build and run with production config
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Apply network limit (requires root)
sudo ./scripts/setup-network-limit.sh

# Check logs
docker-compose logs -f
```

## Monitoring

Check disk usage:
```bash
du -sh ./data/
```

Check container resource usage:
```bash
docker stats dajtu-app-1
```
```

**Step 2: Commit**

```bash
git add docs/DEPLOYMENT.md
git commit -m "docs: add deployment guide with resource limits"
```

---

## Summary

### Różnice Local vs Production

| Zasób | Local | Production |
|-------|-------|------------|
| BASE_URL | http://localhost:8080 | https://dajtu.com |
| MAX_DISK_GB | 10 GB | 50 GB |
| CLEANUP_TARGET_GB | 8 GB | 45 GB |
| CPU limit | brak | 2 cores |
| Memory limit | brak | 2 GB |
| Network limit | brak | 10 Mbps |

### Konfiguracja produkcyjna

| Zasób | Limit | Gdzie skonfigurowane |
|-------|-------|---------------------|
| Dysk | 50 GB | `docker-compose.prod.yml` → `MAX_DISK_GB` |
| Cleanup target | 45 GB | `docker-compose.prod.yml` → `CLEANUP_TARGET_GB` |
| CPU | 2 cores | `docker-compose.prod.yml` → deploy.resources |
| Memory | 2 GB | `docker-compose.prod.yml` → deploy.resources |
| Network | 10 Mbps | `scripts/setup-network-limit.sh` |
| Single file | 20 MB | `docker-compose.yml` → `MAX_FILE_SIZE_MB` |

**Total commits:** 4
