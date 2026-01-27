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
