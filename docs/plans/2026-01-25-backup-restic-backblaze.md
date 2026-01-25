# Backup z Restic + Backblaze B2 - Plan Implementacji

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automatyczny codzienny backup danych dajtu.com (DB + obrazy) na Backblaze B2 z restic.

**Architecture:** Restic na serwerze staging backupuje `/var/www/dajtu/data/` do Backblaze B2. Cron uruchamia backup codziennie o 3:00. SQLite backup przez `.backup` command przed głównym backupem (atomowa kopia). Retencja: 7 daily, 4 weekly, 3 monthly.

**Tech Stack:** restic, Backblaze B2, systemd timer (lub cron), SQLite CLI

---

## Informacje wstępne

### Co backupujemy
- `/var/www/dajtu/data/dajtu.db` - baza SQLite (~100 MB)
- `/var/www/dajtu/data/images/` - obrazy (~40-50 GB)

### Czego NIE backupujemy
- WAL/SHM files (tworzymy atomową kopię DB przed backupem)
- Logi (jeśli będą)

### Dane do konfiguracji (uzupełnić)
```
B2_ACCOUNT_ID=<twój_account_id>
B2_ACCOUNT_KEY=<twój_application_key>
B2_BUCKET=<nazwa_bucketa>
RESTIC_PASSWORD=<silne_hasło_do_repo>
```

---

## Task 1: Instalacja restic na staging

**Files:**
- Brak plików do utworzenia - instalacja pakietu

**Step 1: SSH na staging i zainstaluj restic**

```bash
ssh staging "sudo apt update && sudo apt install -y restic sqlite3"
```

Expected: `restic is already the newest version` lub instalacja bez błędów

**Step 2: Sprawdź wersję**

```bash
ssh staging "restic version"
```

Expected: `restic 0.15.x` lub nowszy

**Step 3: Commit (brak zmian w repo)**

Brak - instalacja na serwerze.

---

## Task 2: Konfiguracja credentials na staging

**Files:**
- Create: `/var/www/dajtu/backup/.env` (na staging, nie w repo!)

**Step 1: Utwórz katalog backup na staging**

```bash
ssh staging "mkdir -p /var/www/dajtu/backup && chmod 700 /var/www/dajtu/backup"
```

**Step 2: Utwórz plik .env z credentials**

```bash
ssh staging "cat > /var/www/dajtu/backup/.env << 'EOF'
export B2_ACCOUNT_ID=<UZUPEŁNIJ>
export B2_ACCOUNT_KEY=<UZUPEŁNIJ>
export RESTIC_REPOSITORY=b2:<BUCKET_NAME>:dajtu
export RESTIC_PASSWORD=<SILNE_HASŁO>
EOF"
```

**Step 3: Zabezpiecz plik**

```bash
ssh staging "chmod 600 /var/www/dajtu/backup/.env"
```

**Step 4: Weryfikuj**

```bash
ssh staging "ls -la /var/www/dajtu/backup/.env"
```

Expected: `-rw------- 1 root root ... .env`

---

## Task 3: Inicjalizacja repozytorium restic

**Step 1: Załaduj credentials i zainicjuj repo**

```bash
ssh staging "source /var/www/dajtu/backup/.env && restic init"
```

Expected:
```
created restic repository ... at b2:<bucket>:dajtu
```

**Step 2: Sprawdź połączenie**

```bash
ssh staging "source /var/www/dajtu/backup/.env && restic snapshots"
```

Expected: Pusta lista snapshotów (bez błędów)

---

## Task 4: Skrypt backupu

**Files:**
- Create: `scripts/backup.sh` (w repo)
- Create: `/var/www/dajtu/scripts/backup.sh` (deploy na staging)

**Step 1: Utwórz skrypt backup.sh**

```bash
#!/bin/bash
set -euo pipefail

# Konfiguracja
BACKUP_DIR="/var/www/dajtu/backup"
DATA_DIR="/var/www/dajtu/data"
LOG_FILE="$BACKUP_DIR/backup.log"

# Załaduj credentials
source "$BACKUP_DIR/.env"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "=== BACKUP START ==="

# 1. Atomowa kopia SQLite
log "Creating SQLite backup..."
DB_BACKUP="$BACKUP_DIR/dajtu.db.backup"
sqlite3 "$DATA_DIR/dajtu.db" ".backup '$DB_BACKUP'"
log "SQLite backup created: $(du -h "$DB_BACKUP" | cut -f1)"

# 2. Restic backup
log "Starting restic backup..."
restic backup \
    --verbose \
    --exclude="*.db-wal" \
    --exclude="*.db-shm" \
    "$DB_BACKUP" \
    "$DATA_DIR/images" \
    2>&1 | tee -a "$LOG_FILE"

# 3. Cleanup starej kopii DB
rm -f "$DB_BACKUP"

# 4. Retencja: 7 daily, 4 weekly, 3 monthly
log "Applying retention policy..."
restic forget \
    --keep-daily 7 \
    --keep-weekly 4 \
    --keep-monthly 3 \
    --prune \
    2>&1 | tee -a "$LOG_FILE"

log "=== BACKUP COMPLETE ==="
```

Zapisz do: `scripts/backup.sh`

**Step 2: Skopiuj na staging**

```bash
scp scripts/backup.sh staging:/var/www/dajtu/scripts/
ssh staging "chmod +x /var/www/dajtu/scripts/backup.sh"
```

**Step 3: Test ręczny (pierwszy backup)**

```bash
ssh staging "sudo /var/www/dajtu/scripts/backup.sh"
```

Expected: Backup wykonany bez błędów, snapshot utworzony.

**Step 4: Weryfikuj snapshot**

```bash
ssh staging "source /var/www/dajtu/backup/.env && restic snapshots"
```

Expected: Jeden snapshot z aktualną datą.

**Step 5: Commit**

```bash
git add scripts/backup.sh
git commit -m "feat: add restic backup script for Backblaze B2"
```

---

## Task 5: Systemd timer (automatyczny backup)

**Files:**
- Create: `scripts/dajtu-backup.service` (w repo)
- Create: `scripts/dajtu-backup.timer` (w repo)

**Step 1: Utwórz service unit**

```ini
# scripts/dajtu-backup.service
[Unit]
Description=Dajtu backup to Backblaze B2
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/var/www/dajtu/scripts/backup.sh
User=root
Nice=19
IOSchedulingClass=idle

[Install]
WantedBy=multi-user.target
```

**Step 2: Utwórz timer unit**

```ini
# scripts/dajtu-backup.timer
[Unit]
Description=Daily dajtu backup at 3:00 AM

[Timer]
OnCalendar=*-*-* 03:00:00
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
```

**Step 3: Deploy na staging**

```bash
scp scripts/dajtu-backup.service scripts/dajtu-backup.timer staging:/tmp/
ssh staging "sudo mv /tmp/dajtu-backup.* /etc/systemd/system/ && \
             sudo systemctl daemon-reload && \
             sudo systemctl enable --now dajtu-backup.timer"
```

**Step 4: Weryfikuj timer**

```bash
ssh staging "systemctl list-timers dajtu-backup.timer"
```

Expected: Timer aktywny, następne uruchomienie ~3:00.

**Step 5: Commit**

```bash
git add scripts/dajtu-backup.service scripts/dajtu-backup.timer
git commit -m "feat: add systemd timer for daily backup at 3 AM"
```

---

## Task 6: Skrypt restore (disaster recovery)

**Files:**
- Create: `scripts/restore.sh`

**Step 1: Utwórz skrypt restore**

```bash
#!/bin/bash
set -euo pipefail

# Konfiguracja
BACKUP_DIR="/var/www/dajtu/backup"
DATA_DIR="/var/www/dajtu/data"
RESTORE_DIR="/var/www/dajtu/restore-tmp"

source "$BACKUP_DIR/.env"

echo "=== DAJTU RESTORE ==="
echo ""

# Lista snapshotów
echo "Available snapshots:"
restic snapshots
echo ""

read -p "Enter snapshot ID to restore (or 'latest'): " SNAPSHOT_ID

# Restore do tymczasowego katalogu
echo "Restoring to $RESTORE_DIR..."
rm -rf "$RESTORE_DIR"
mkdir -p "$RESTORE_DIR"

restic restore "$SNAPSHOT_ID" --target "$RESTORE_DIR"

echo ""
echo "Restored files:"
ls -la "$RESTORE_DIR"
echo ""

read -p "Apply restore? This will REPLACE current data! (yes/no): " CONFIRM

if [ "$CONFIRM" = "yes" ]; then
    echo "Stopping dajtu container..."
    cd /var/www/dajtu && docker compose stop app

    echo "Replacing data..."
    # Backup obecnych danych na wszelki wypadek
    mv "$DATA_DIR" "${DATA_DIR}.old.$(date +%Y%m%d%H%M%S)"

    # Przenieś restored data
    mkdir -p "$DATA_DIR"

    # DB z backup/
    if [ -f "$RESTORE_DIR/var/www/dajtu/backup/dajtu.db.backup" ]; then
        cp "$RESTORE_DIR/var/www/dajtu/backup/dajtu.db.backup" "$DATA_DIR/dajtu.db"
    fi

    # Images
    if [ -d "$RESTORE_DIR/var/www/dajtu/data/images" ]; then
        cp -r "$RESTORE_DIR/var/www/dajtu/data/images" "$DATA_DIR/"
    fi

    echo "Starting dajtu container..."
    docker compose up -d app

    echo "Cleaning up..."
    rm -rf "$RESTORE_DIR"

    echo "=== RESTORE COMPLETE ==="
else
    echo "Restore cancelled. Files remain in $RESTORE_DIR"
fi
```

**Step 2: Deploy i test**

```bash
scp scripts/restore.sh staging:/var/www/dajtu/scripts/
ssh staging "chmod +x /var/www/dajtu/scripts/restore.sh"
```

**Step 3: Commit**

```bash
git add scripts/restore.sh
git commit -m "feat: add restore script for disaster recovery"
```

---

## Task 7: Dokumentacja w CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Dodaj sekcję Backup**

Dodaj po sekcji "Troubleshooting":

```markdown
## Backup

**Backend:** Restic → Backblaze B2
**Harmonogram:** Codziennie o 3:00 (systemd timer)
**Retencja:** 7 daily, 4 weekly, 3 monthly

### Komendy

```bash
# Status backupu
ssh staging "systemctl status dajtu-backup.timer"

# Ręczny backup
ssh staging "sudo /var/www/dajtu/scripts/backup.sh"

# Lista snapshotów
ssh staging "source /var/www/dajtu/backup/.env && restic snapshots"

# Sprawdź integralność
ssh staging "source /var/www/dajtu/backup/.env && restic check"

# Restore (interaktywny)
ssh staging "sudo /var/www/dajtu/scripts/restore.sh"

# Logi ostatniego backupu
ssh staging "tail -100 /var/www/dajtu/backup/backup.log"
```

### Pliki na staging

- `/var/www/dajtu/backup/.env` - credentials (B2 + restic password)
- `/var/www/dajtu/backup/backup.log` - logi backupów
- `/var/www/dajtu/scripts/backup.sh` - skrypt backupu
- `/var/www/dajtu/scripts/restore.sh` - skrypt restore

### Co jest backupowane

- `dajtu.db` - atomowa kopia SQLite (przez `.backup`)
- `data/images/` - wszystkie obrazy

**Uwaga:** WAL/SHM nie są backupowane - używamy `.backup` SQLite dla spójności.
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add backup documentation"
```

---

## Task 8: Weryfikacja końcowa

**Step 1: Sprawdź wszystkie elementy**

```bash
# Timer aktywny
ssh staging "systemctl is-active dajtu-backup.timer"

# Credentials zabezpieczone
ssh staging "ls -la /var/www/dajtu/backup/.env"

# Skrypty executable
ssh staging "ls -la /var/www/dajtu/scripts/backup.sh /var/www/dajtu/scripts/restore.sh"

# Minimum 1 snapshot istnieje
ssh staging "source /var/www/dajtu/backup/.env && restic snapshots --json | jq length"
```

**Step 2: Test integralności**

```bash
ssh staging "source /var/www/dajtu/backup/.env && restic check"
```

Expected: `no errors were found`

**Step 3: Push wszystko**

```bash
git push
```

---

## Podsumowanie

| Element | Lokalizacja | Notatka |
|---------|-------------|---------|
| Credentials | `/var/www/dajtu/backup/.env` | NIE w repo! |
| Skrypt backup | `scripts/backup.sh` | W repo |
| Skrypt restore | `scripts/restore.sh` | W repo |
| Systemd units | `scripts/dajtu-backup.*` | W repo, deployed to /etc/systemd/system/ |
| Logi | `/var/www/dajtu/backup/backup.log` | Na staging |
| Dokumentacja | `CLAUDE.md` | W repo |

**Koszty szacunkowe (Backblaze B2):**
- Storage: ~$0.25/GB/miesiąc → ~50 GB = ~$12.50/miesiąc
- Download: $0.01/GB (tylko przy restore)
- API calls: praktycznie darmowe

**Czas restore:** ~1-2h dla 50 GB (zależnie od łącza)
