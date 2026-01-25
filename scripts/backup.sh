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

# 4. Retencja: 24 hourly, 7 daily, 4 weekly, 3 monthly
log "Applying retention policy..."
restic forget \
    --keep-hourly 24 \
    --keep-daily 7 \
    --keep-weekly 4 \
    --keep-monthly 3 \
    --prune \
    2>&1 | tee -a "$LOG_FILE"

log "=== BACKUP COMPLETE ==="
