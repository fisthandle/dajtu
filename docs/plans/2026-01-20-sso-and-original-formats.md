# SSO Braterstwo + Original Formats Implementation Plan (updated)

**Goal:** Dodać SSO z Braterstwa oraz zachowywać pliki w oryginalnych formatach obok WebP.

**Wymagania doprecyzowane:**
- Format oryginału: `orig_{name}.{ext}` (np. `orig_original.jpg`).
- `file_size` ma uwzględniać rozmiar oryginału.
- SSO zgodne z `BratController.php` z `dev/ogloszenia`.
- Dodać endpoint `/u/{slug}`.
- Tryb fail-fast dla błędnej konfiguracji SSO.

**Architecture:**
- SSO: handler `/auth/brat/{data}` dekoduje payload AES-256-CBC + weryfikuje HMAC.
- Oryginalne formaty: filesystem przechowuje `orig_{name}.{ext}` obok WebP.
- Testy: rozszerzenie istniejącej infrastruktury testowej.

**Tech Stack:** Go, crypto/aes, crypto/hmac, SQLite, bimg.

---

## Task 1: Konfiguracja SSO + Original Formats

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Zakres:**
- Dodaj pola SSO (zgodne z `BratController.php`):
  - `BratHashSecret`, `BratEncryptionKey`, `BratEncryptionIV`, `BratCipher`
  - `BratMaxSkewSeconds`, `BratHashLength`, `BratHashBytes`
  - `BratMaxPseudonimBytes`
- Dodaj opcję `KeepOriginalFormat` (domyślnie `true`).
- Dodaj helper `getEnvBool`.

**Testy:**
- Rozszerz istniejące `config_test.go` o:
  - `TestLoadSSO_FromEnv`
  - `TestLoadSSO_Defaults` (z `BratMaxPseudonimBytes`)
  - `TestLoadOriginalFormats_Default`

---

## Task 2: Moduł dekodowania SSO (zgodny z BratController.php)

**Files:**
- Create: `internal/auth/brat.go`
- Create: `internal/auth/brat_test.go`

**Założenia (na podstawie `BratController.php`):**
- `data` jest `rawurldecode` + base64 URL-safe (`-`/`_`) z paddingiem do 4.
- AES-256-CBC, klucz = SHA256(encryptionKey) **raw bytes**.
- Payload binarny:
  - `[4B timestamp][8B punktacja][1B len][NB pseudonim][hashBytes HMAC]`
- Hash: `hex(hmac_sha256("timestamp|punktacja|pseudonim", hashSecret))[:hashLength]`.
- `hashLength == hashBytes*2` (fail-fast).
- `maxPseudonimBytes` limit.
- `maxSkewSeconds <= 0` = brak limitu.

**Fail-fast:**
`NewBratDecoder(cfg)` zwraca `error`, jeśli:
- brakuje wymaganych pól,
- `hashLength != hashBytes*2`,
- `IV` ma złą długość dla wybranego szyfru.

**Testy:**
- Invalid config (hash length mismatch, IV length mismatch).
- Invalid base64.
- Valid payload (helper do wygenerowania zaszyfrowanego payloadu).

---

## Task 3: DB i model użytkownika (Brat)

**Files:**
- Modify: `internal/storage/db.go`
- Modify: `internal/storage/db_test.go`

**Zakres:**
- Rozszerz `users` o:
  - `brat_pseudo TEXT UNIQUE`
  - `brat_punktacja INTEGER NOT NULL DEFAULT 0`
  - `updated_at INTEGER NOT NULL DEFAULT 0`
- Zapewnij migracje dla istniejących DB:
  - `PRAGMA table_info` + `ALTER TABLE` dla brakujących kolumn.
  - `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_brat_pseudo`.
  - Backfill `updated_at` = `created_at` gdy `0`.
- Rozszerz `User` struct o `BratPseudo`, `BratPunktacja`, `UpdatedAt`.
- Zaktualizuj `InsertUser` i `GetUserBySlug`.
- Dodaj `GetOrCreateBratUser(pseudonim, punktacja)`:
  - `SELECT` po `brat_pseudo`.
  - Jeśli istnieje: update punktacji + `updated_at`.
  - Jeśli brak (`sql.ErrNoRows`): nowy user z unikalnym slugiem.

**Testy:**
- `TestGetOrCreateBratUser_New`
- `TestGetOrCreateBratUser_Existing`
- Sprawdź, że `InsertUser` działa z nowym schematem.

---

## Task 4: Handlery SSO + /u

**Files:**
- Create: `internal/handler/auth.go`
- Create: `internal/handler/user.go`
- Add template: `internal/handler/templates/user.html`
- Create/Modify: `internal/handler/auth_test.go`, `internal/handler/user_test.go`

**Zakres:**
- `AuthHandler`:
  - `HandleBratSSO`:
    - pobiera `data` z path lub query,
    - dekoduje payload, tworzy/aktualizuje użytkownika,
    - JSON (gdy `Accept: application/json`) albo redirect do `/u/{slug}`.
  - Decoder inicjalizowany w `NewAuthHandler` (fail-fast dla błędnej konfiguracji).
- `UserHandler`:
  - `GET /u/{slug}` -> prosta strona profilu (pseudonim + punktacja).

**Testy:**
- `TestAuthHandler_MissingData` -> 400.
- `TestAuthHandler_SSODisabled` -> 503.
- `TestUserHandler_NotFound` -> 404.

---

## Task 5: Routing

**Files:**
- Modify: `cmd/dajtu/main.go`

**Zakres:**
- Zainicjalizuj `authHandler`, `userHandler`.
- Podłącz routing:
  - `/auth/brat/` -> `authHandler.HandleBratSSO`
  - `/u/` -> `userHandler.View`
- Jeśli SSO skonfigurowane, ale `NewBratDecoder` zwraca błąd -> `log.Fatal` (fail-fast).

---

## Task 6: Filesystem - oryginalne formaty

**Files:**
- Modify: `internal/storage/filesystem.go`
- Modify/Create: `internal/storage/filesystem_test.go`

**Zakres:**
- Mapowanie MIME -> ext: `.jpg`, `.png`, `.gif`, `.webp`, `.avif`.
- `SaveOriginal(slug, name, data, mime)` zapisuje `orig_{name}.{ext}`.
- `GetOriginalPath(slug, name)` zwraca ścieżkę dla `orig_{name}.*` (iteruje po znanych ext).

**Testy:**
- `TestSaveOriginal_JPEG`
- `TestSaveOriginal_PNG`
- `TestGetOriginalPath`

---

## Task 7: Upload + Gallery - zapis oryginału

**Files:**
- Modify: `internal/handler/upload.go`
- Modify: `internal/handler/gallery.go`
- Modify: `internal/handler/upload_test.go`

**Zakres:**
- Po `ValidateAndDetect`, przed `Process`:
  - jeśli `KeepOriginalFormat`, zapisz oryginał (`orig_original.<ext>`),
  - dodaj jego rozmiar do `totalSize`.
- Użyj `format` z `ValidateAndDetect` jako MIME.

**Testy:**
- `TestUploadHandler_SavesOriginal` (sprawdza istnienie pliku).

---

## Task 8: Endpoint do pobierania oryginałów

**Files:**
- Modify: `cmd/dajtu/main.go`
- Modify: `internal/handler/upload.go` (lub nowy handler)

**Zakres:**
- `/i/{slug}/original` serwuje `orig_original.<ext>`.
- `ServeOriginal(w,r,slug)`:
  - `GetOriginalPath`,
  - `Content-Type` po rozszerzeniu,
  - cache headers.

---

## Task 9: Test plan + .env.example

**Files:**
- Modify: `docs/plans/2026-01-20-comprehensive-tests.md`
- Create: `.env.example`
- Modify: `README.md` (jeśli istnieje)

**Zakres:**
- Dodaj sekcje testów SSO i original formats.
- `.env.example` uwzględnia:
  - `BRAT_MAX_PSEUDONIM_BYTES`
  - `KEEP_ORIGINAL_FORMAT`

---

## Zależności

- Tasks 1-5 (SSO) sekwencyjne.
- Tasks 6-8 (original formats) sekwencyjne.
- Task 9 na końcu.
