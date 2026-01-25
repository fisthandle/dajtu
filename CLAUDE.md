# Dajtu - Image Hosting Service

## Deploy

```bash
# Commit + push + deploy
ssh staging "cd /var/www/dajtu && git pull && docker compose up --build -d"

# Tylko restart (bez rebuild)
ssh staging "cd /var/www/dajtu && docker compose up -d --force-recreate"

# Logi
ssh staging "docker logs dajtu_app --tail 100 -f"
```

**Serwer:** staging (SSH config)
**Ścieżka:** `/var/www/dajtu`
**URL:** https://dajtu.com

## Stack

- Go 1.22
- SQLite (data/dajtu.db)
- libvips (przetwarzanie obrazów)
- Docker + Caddy (reverse proxy w /var/www/infra)

## Konfiguracja

### Env vars (wszystkie w docker-compose.yml na staging)

| Zmienna | Default | Opis |
|---------|---------|------|
| `PORT` | 8080 | Port HTTP |
| `DATA_DIR` | ./data | Katalog storage |
| `BASE_URL` | (pusty) | URL publiczny do linków |
| `MAX_FILE_SIZE_MB` | 20 | Max rozmiar pliku |
| `MAX_DISK_GB` | 50 | Limit dysku (wyzwala cleanup) |
| `CLEANUP_TARGET_GB` | 45 | Cel po cleanup |
| `KEEP_ORIGINAL_FORMAT` | true | Zachowaj oryginał oprócz WebP |

### Access Control

| Zmienna | Default | Opis |
|---------|---------|------|
| `ALLOWED_ORIGINS` | (pusty) | CORS - dozwolone domeny (csv). **Pusty = wszystkie dozwolone** |
| `PUBLIC_UPLOAD` | true | Upload bez logowania |
| `ADMIN_NICKS` | KS Amator,gruby wonsz | Lista nicków adminów (csv) |

### SSO Braterstwo (BRAT_*)

| Zmienna | Opis |
|---------|------|
| `BRAT_HASH_SECRET` | Klucz HMAC do weryfikacji |
| `BRAT_ENCRYPTION_KEY` | Klucz AES do dekrypcji tokenu |
| `BRAT_ENCRYPTION_IV` | IV dla AES |
| `BRAT_CIPHER` | Algorytm (domyślnie AES-256-CBC) |
| `BRAT_MAX_SKEW_SECONDS` | Max różnica czasu tokenu (600s) |

## Generowane obrazki

Format: **WebP** (quality 90, thumb 85)

| Rozmiar | Szerokość | Użycie |
|---------|-----------|--------|
| `original` | max 4096px | pełna jakość |
| `1920` | 1920px | desktop |
| `800` | 800px | mobile |
| `200` | 200px | lista/grid |
| `thumb` | 100x100px | miniatura (crop) |

**Oryginał:** jeśli `KEEP_ORIGINAL_FORMAT=true`, zachowuje też plik w oryginalnym formacie (JPG/PNG/GIF)

### URL obrazków

```
https://dajtu.com/i/{slug}           # podgląd HTML
https://dajtu.com/i/{slug}/original  # WebP pełny
https://dajtu.com/i/{slug}/1920      # WebP 1920px
https://dajtu.com/i/{slug}/800       # WebP 800px
https://dajtu.com/i/{slug}/200       # WebP 200px
https://dajtu.com/i/{slug}/thumb     # WebP 100x100
```

Slug: 5-znakowy hash (np. `ab1c2`)

## Endpoints

| Path | Metoda | Opis |
|------|--------|------|
| `/` | GET | Strona główna |
| `/upload` | POST | Upload publiczny (jeśli PUBLIC_UPLOAD=true) |
| `/brtup/{token}/{id}/{title}` | POST | Upload SSO Braterstwo |
| `/brrrt` | GET | SSO callback z Braterstwa |
| `/i/{slug}` | GET | Podgląd obrazka (HTML) |
| `/i/{slug}/{size}` | GET | Obrazek WebP w rozmiarze |
| `/i/{slug}/original` | GET | Oryginał w formacie źródłowym (jeśli zachowany) |
| `/g/{slug}` | GET | Galeria |

## Admin Panel

Panel dostępny dla użytkowników z listy `ADMIN_NICKS` (domyślnie: "KS Amator", "gruby wonsz").

| Endpoint | Opis |
|----------|------|
| `/admin` | Dashboard ze statystykami |
| `/admin/users` | Lista kont |
| `/admin/galleries` | Lista galerii (z delete) |
| `/admin/images` | Lista zdjęć (z delete i licznikiem pobrań) |

## Lokalne uruchomienie

```bash
# Kompilacja i uruchomienie
go build -o dajtu ./cmd/dajtu && ./dajtu
```

**Konfiguracja:** Aplikacja automatycznie ładuje `.env` z katalogu głównego (symlink do `config/.env`).

```bash
# Struktura
config/.env      # właściwy plik z konfiguracją (gitignored)
.env -> config/.env   # symlink
```

**Uwaga:** Bez zmiennych `BRAT_*` serwer nie wystartuje.

## Logi

Aplikacja używa standardowego `log` package (stderr).

**Lokalnie:** logi widoczne w terminalu gdzie uruchomiono `./dajtu`

**Staging:** `docker logs dajtu_app --tail 100 -f`

```bash
# Filtrowanie błędów
ssh staging "docker logs dajtu_app 2>&1 | grep -i error"

# Logi z ostatniej godziny
ssh staging "docker logs dajtu_app --since 1h"
```

### TODO: Logi do pliku
Rozważyć przekierowanie stderr do `data/logs/` przy deployu:
- Rotacja logów (dzienne pliki lub max rozmiar)
- Łatwiejsze przeszukiwanie historii
- Opcja: slog (Go stdlib) dla structured logging (JSON)

## Troubleshooting

### CORS blokuje requesty z lokalnego dev
Ustaw `ALLOWED_ORIGINS=` (pusty) lub zakomentuj w docker-compose.yml - wtedy przepuści wszystkie origin

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
