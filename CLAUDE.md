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


Lokalnie:
go build -o dajtu ./cmd/dajtu && ./dajtu


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
| `LOG_DIR` | ./data/logs | Katalog logów |
| `CACHE_DIR` | /tmp/dajtu-cache | Katalog cache obrazków (on-demand) |

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

### Przy uploading (statyczne)

| Rozmiar | Szerokość | Użycie |
|---------|-----------|--------|
| `original` | max 4096px | pełna jakość |
| `thumb` | 200x200px | miniatura (crop) |

### On-demand (dynamiczne + cache)

| Rozmiar | Szerokość |
|---------|-----------|
| `800` | 800px |
| `1200` | 1200px |
| `1600` | 1600px |
| `2400` | 2400px |

Dynamiczne rozmiary generowane przy pierwszym request i cache'owane w docker volume `dajtu-cache`.

**Oryginał:** jeśli `KEEP_ORIGINAL_FORMAT=true`, zachowuje też plik w oryginalnym formacie (JPG/PNG/GIF)

### URL obrazków

```
https://dajtu.com/i/{slug}           # podgląd HTML
https://dajtu.com/i/{slug}/original  # WebP pełny
https://dajtu.com/i/{slug}/800       # WebP 800px (on-demand)
https://dajtu.com/i/{slug}/1200      # WebP 1200px (on-demand)
https://dajtu.com/i/{slug}/1600      # WebP 1600px (on-demand)
https://dajtu.com/i/{slug}/2400      # WebP 2400px (on-demand)
https://dajtu.com/i/{slug}/thumb     # WebP 200x200
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

Structured logging do plików + stdout (dual output).

**Lokalizacja:** `data/logs/{YY.MM}_{category}.log`

**Kategorie:**
- `app` - główny logger aplikacji
- `requests` - HTTP requesty (method, path, status, duration)
- `cache` - cache hit/miss
- `image` - operacje resize
- `upload` - uploady plików

**Rotacja:** miesięczna (po nazwie pliku)

### Komendy

```bash
# Docker logs (stdout)
ssh staging "docker logs dajtu_app --tail 100 -f"

# Pliki logów
ssh staging "ls -la /var/www/dajtu/data/logs/"
ssh staging "tail -100 /var/www/dajtu/data/logs/26.01_requests.log"
```

### Panel admina

Logi dostępne w `/admin/logs`

## Troubleshooting

### CORS blokuje requesty z lokalnego dev
Ustaw `ALLOWED_ORIGINS=` (pusty) lub zakomentuj w docker-compose.yml - wtedy przepuści wszystkie origin

### Logi nie zapisują się do plików
Aplikacja w kontenerze działa jako user `dajtu` (uid 999). Katalog `data/logs/` musi mieć odpowiednie uprawnienia:
```bash
ssh staging "sudo chown -R 999:999 /var/www/dajtu/data/logs"
```

### Cache nie działa (write=fail w logach)
Cache jest w docker volume `dajtu-cache`. Volume musi mieć owner uid 999:
```bash
ssh staging "sudo chown -R 999:999 /var/lib/docker/volumes/dajtu_dajtu-cache/_data"
```
Sprawdź czy volume istnieje:
```bash
ssh staging "docker volume ls | grep dajtu"
```
Cleanup cache (codziennie o 4:00 przez systemd timer):
```bash
ssh staging "systemctl status dajtu-cache-cleanup.timer"
```

## Backup

**Backend:** Restic → Backblaze B2
**Harmonogram:** Co godzinę (systemd timer)
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
