#!/bin/bash
set -euo pipefail

# Lokalne ścieżki
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DATA_DIR="$PROJECT_DIR/data"
RESTORE_DIR="$PROJECT_DIR/.restore-tmp"
CREDENTIALS="$PROJECT_DIR/config/.env.backup"

# Załaduj credentials
if [ ! -f "$CREDENTIALS" ]; then
    echo "Brak pliku credentials: $CREDENTIALS"
    echo "Utwórz go z zawartością:"
    echo "  export B2_ACCOUNT_ID=..."
    echo "  export B2_ACCOUNT_KEY=..."
    echo "  export RESTIC_REPOSITORY=b2:bucket:path"
    echo "  export RESTIC_PASSWORD=..."
    exit 1
fi
source "$CREDENTIALS"

echo "=== RESTORE DO LOKALNEGO DEV ==="
echo ""
echo "Projekt: $PROJECT_DIR"
echo "Data dir: $DATA_DIR"
echo ""

# Sprawdź czy istnieje poprzedni restore
if [ -d "$RESTORE_DIR" ]; then
    echo "Znaleziono poprzednio pobrany backup w: $RESTORE_DIR"
    ls -la "$RESTORE_DIR"
    echo ""
    read -p "Użyć istniejącego? (yes = użyj, no = pobierz nowy): " USE_EXISTING

    if [ "$USE_EXISTING" = "yes" ]; then
        echo "Używam istniejącego backupu..."
    else
        # Lista snapshotów
        echo "Dostępne snapshoty:"
        restic snapshots
        echo ""

        read -p "Snapshot ID (lub 'latest'): " SNAPSHOT_ID

        # Restore do tmp
        echo "Pobieram snapshot $SNAPSHOT_ID..."
        rm -rf "$RESTORE_DIR"
        mkdir -p "$RESTORE_DIR"
        restic restore "$SNAPSHOT_ID" --target "$RESTORE_DIR"
    fi
else
    # Lista snapshotów
    echo "Dostępne snapshoty:"
    restic snapshots
    echo ""

    read -p "Snapshot ID (lub 'latest'): " SNAPSHOT_ID

    # Restore do tmp
    echo "Pobieram snapshot $SNAPSHOT_ID..."
    mkdir -p "$RESTORE_DIR"
    restic restore "$SNAPSHOT_ID" --target "$RESTORE_DIR"
fi

echo ""
echo "Pobrane pliki:"
ls -la "$RESTORE_DIR"
echo ""

read -p "Zastąpić lokalne data/? (yes/no): " CONFIRM

if [ "$CONFIRM" = "yes" ]; then
    # Backup obecnych danych
    if [ -d "$DATA_DIR" ]; then
        BACKUP_NAME="${DATA_DIR}.old.$(date +%Y%m%d%H%M%S)"
        echo "Backup obecnych danych: $BACKUP_NAME"
        mv "$DATA_DIR" "$BACKUP_NAME"
    fi

    mkdir -p "$DATA_DIR"

    # DB
    if [ -f "$RESTORE_DIR/var/www/dajtu/backup/dajtu.db.backup" ]; then
        cp "$RESTORE_DIR/var/www/dajtu/backup/dajtu.db.backup" "$DATA_DIR/dajtu.db"
        echo "✓ Baza danych przywrócona"
    fi

    # Images
    if [ -d "$RESTORE_DIR/var/www/dajtu/data/images" ]; then
        cp -r "$RESTORE_DIR/var/www/dajtu/data/images" "$DATA_DIR/"
        echo "✓ Obrazy przywrócone"
    fi

    rm -rf "$RESTORE_DIR"

    echo ""
    echo "=== GOTOWE ==="
    echo "Możesz uruchomić: go build -o dajtu ./cmd/dajtu && ./dajtu"
else
    echo "Anulowano. Pliki pozostają w: $RESTORE_DIR"
fi
