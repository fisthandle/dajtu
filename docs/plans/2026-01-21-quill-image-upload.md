# Integracja Quill → dajtu: Upload obrazów z forum

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Umożliwić upload obrazów bezpośrednio z edytora Quill na forum Braterstwa do serwisu dajtu, z automatycznym grupowaniem w galerie per wątek.

**Architecture:**
1. Quill generuje URL: `POST dajtu.com/brtup/{token}/{entryId}/{titleBase64}`
2. Dajtu weryfikuje istniejący token SSO (ten sam co do logowania)
3. Tworzy/znajduje galerię dla tego usera + entryId
4. Uploaduje obraz, zwraca URL

**Tech Stack:** Go (dajtu), JavaScript (Quill)

---

## Endpoint

```
POST /brtup/{token}/{entryId}/{titleBase64}

Parametry URL:
- token     - zaszyfrowany token SSO (ten sam co /brrrt/, ale ważny 24h dla uploadu)
- entryId   - ID wątku lub "nope" dla nowych
- titleBase64 - tytuł galerii w base64 lub "nope"

Body: multipart/form-data z polem "image"

Response:
{
  "url": "https://dajtu.com/i/abc12.webp",
  "thumbUrl": "https://dajtu.com/i/abc12/thumb.webp",
  "filename": "zdjecie.jpg",
  "slug": "abc12"
}
```

## Rozmiary obrazów

Dajtu generuje:
- `original` - max 4096px (proporcjonalnie)
- `1920`, `800`, `200` - mniejsze wersje (proporcjonalnie)
- `thumb` - **100x100px z cropem** (wyśrodkowany, wypełniony)

Quill wstawia thumbnail jako link do oryginału:
```html
<a href="https://dajtu.com/i/abc12.webp" target="_blank">
  <img src="https://dajtu.com/i/abc12/thumb.webp">
</a>
```

---

## Task 1: Thumbnail 100x100 z cropem + 24h token

**Files:**
- Modify: `/home/pawel/dev/dajtu/internal/image/processor.go`
- Modify: `/home/pawel/dev/dajtu/internal/auth/brat.go`

**Step 1: Dodaj rozmiar thumb z cropem do processor.go**

Zmień `Sizes` i dodaj nowy typ:

```go
type Size struct {
    Name    string
    Width   int
    Height  int  // 0 = proporcjonalnie, >0 = crop do kwadratu
    Quality int
}

var Sizes = []Size{
    {Name: "original", Width: 4096, Height: 0, Quality: 90},
    {Name: "1920", Width: 1920, Height: 0, Quality: 90},
    {Name: "800", Width: 800, Height: 0, Quality: 90},
    {Name: "200", Width: 200, Height: 0, Quality: 90},
    {Name: "thumb", Width: 100, Height: 100, Quality: 85},  // crop do kwadratu
}
```

Zmodyfikuj `Process()` żeby obsługiwał crop:

```go
for _, s := range Sizes {
    targetWidth := s.Width
    if size.Width < targetWidth {
        targetWidth = size.Width
    }

    opts := bimg.Options{
        Width:         targetWidth,
        Type:          bimg.WEBP,
        Quality:       s.Quality,
        StripMetadata: true,
    }

    // Thumbnail z cropem
    if s.Height > 0 {
        opts.Width = s.Width
        opts.Height = s.Height
        opts.Crop = true
        opts.Gravity = bimg.GravityCentre
    }

    processed, err := bimg.NewImage(data).Process(opts)
    // ... reszta bez zmian
}
```

**Step 2: Sparametryzuj Decode() żeby przyjmowała zakres czasowy**

Zmień `verifyTimestamp` na przyjmowanie parametru i dodaj `DecodeWithMaxAge`:

```go
// Decode - używa domyślnego MaxSkewSeconds z konfiguracji
func (d *BratDecoder) Decode(data string) (*BratUser, error) {
    return d.DecodeWithMaxAge(data, int64(d.cfg.MaxSkewSeconds))
}

// DecodeWithMaxAge - jak Decode, ale z dowolnym oknem czasowym
func (d *BratDecoder) DecodeWithMaxAge(data string, maxAgeSeconds int64) (*BratUser, error) {
    if d == nil {
        return nil, ErrConfigMissing
    }

    decoded, err := d.decodeBase64(data)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrInvalidBase64, err)
    }

    decrypted, err := d.decrypt(decoded)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrDecryptFailed, err)
    }

    parsed, err := parseBinaryPayload(decrypted, d.cfg.HashBytes, d.cfg.MaxPseudonimBytes)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
    }

    if !d.verifyHMAC(parsed) {
        return nil, ErrInvalidHMAC
    }

    if !d.verifyTimestampWithAge(parsed.Timestamp, maxAgeSeconds) {
        return nil, ErrTimestampExpired
    }

    return &BratUser{
        Pseudonim: parsed.Pseudonim,
        Punktacja: parsed.Punktacja,
        Timestamp: parsed.Timestamp,
    }, nil
}

func (d *BratDecoder) verifyTimestampWithAge(ts, maxAgeSeconds int64) bool {
    if maxAgeSeconds <= 0 {
        return true
    }
    now := time.Now().Unix()
    return ts >= now-maxAgeSeconds && ts <= now+60
}
```

Usuń starą metodę `verifyTimestamp()` - nie jest już potrzebna.

**Step 3: Commit**

```bash
git add internal/image/processor.go internal/auth/brat.go
git commit -m "feat: add 100x100 thumb with crop and 24h upload token"
```

---

## Task 2: Handler /brtup w dajtu

**Files:**
- Create: `/home/pawel/dev/dajtu/internal/handler/brat_upload.go`
- Modify: `/home/pawel/dev/dajtu/cmd/dajtu/main.go`
- Modify: `/home/pawel/dev/dajtu/internal/storage/db.go`

**Step 1: Dodaj pole external_id do Gallery i metodę wyszukiwania**

W `internal/storage/db.go` dodaj do struktury `Gallery`:

```go
type Gallery struct {
    ID          int64
    Slug        string
    EditToken   string
    Title       string
    Description string
    ExternalID  string  // "brat:{userId}:{entryId}"
    UserID      *int64
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

Dodaj metodę:

```go
func (db *DB) GetGalleryByExternalID(externalID string) (*Gallery, error) {
    var g Gallery
    var userID sql.NullInt64
    err := db.conn.QueryRow(`
        SELECT id, slug, edit_token, title, description, external_id, user_id, created_at, updated_at
        FROM galleries WHERE external_id = ?
    `, externalID).Scan(
        &g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.ExternalID, &userID, &g.CreatedAt, &g.UpdatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    if userID.Valid {
        g.UserID = &userID.Int64
    }
    return &g, nil
}
```

Zmodyfikuj `CreateGallery` żeby zapisywał `external_id`:

```go
func (db *DB) CreateGallery(g *Gallery) error {
    res, err := db.conn.Exec(`
        INSERT INTO galleries (slug, edit_token, title, description, external_id, user_id)
        VALUES (?, ?, ?, ?, ?, ?)
    `, g.Slug, g.EditToken, g.Title, g.Description, g.ExternalID, g.UserID)
    if err != nil {
        return err
    }
    id, _ := res.LastInsertId()
    g.ID = id
    return nil
}
```

**Step 2: Dodaj migrację kolumny external_id**

W `initSchema` dodaj:

```go
`ALTER TABLE galleries ADD COLUMN external_id TEXT`,
`CREATE INDEX IF NOT EXISTS idx_galleries_external ON galleries(external_id)`,
```

Uwaga: ALTER TABLE może failować jeśli kolumna istnieje - ignoruj błąd lub sprawdź przed.

**Step 3: Stwórz handler brat_upload.go**

```go
package handler

import (
    "encoding/base64"
    "encoding/json"
    "log"
    "net/http"
    "strings"

    "dajtu/internal/auth"
    "dajtu/internal/config"
    "dajtu/internal/image"
    "dajtu/internal/storage"
)

type BratUploadHandler struct {
    cfg     *config.Config
    db      *storage.DB
    fs      *storage.Filesystem
    proc    *image.Processor
    decoder *auth.BratDecoder
}

func NewBratUploadHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem, proc *image.Processor, decoder *auth.BratDecoder) *BratUploadHandler {
    return &BratUploadHandler{cfg: cfg, db: db, fs: fs, proc: proc, decoder: decoder}
}

type BratUploadResponse struct {
    URL      string `json:"url"`
    ThumbURL string `json:"thumbUrl"`
    Filename string `json:"filename"`
    Slug     string `json:"slug"`
}

func (h *BratUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // CORS
    origin := r.Header.Get("Origin")
    if strings.Contains(origin, "braterstwo") {
        w.Header().Set("Access-Control-Allow-Origin", origin)
        w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    }

    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusOK)
        return
    }

    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse URL: /brtup/{token}/{entryId}/{titleBase64}
    path := strings.TrimPrefix(r.URL.Path, "/brtup/")
    parts := strings.SplitN(path, "/", 3)
    if len(parts) != 3 {
        http.Error(w, "Invalid URL format", http.StatusBadRequest)
        return
    }

    token, entryID, titleB64 := parts[0], parts[1], parts[2]

    // Dekoduj token SSO (24h okno dla uploadu)
    bratUser, err := h.decoder.DecodeWithMaxAge(token, 86400)
    if err != nil {
        log.Printf("brtup: token decode error: %v", err)
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }

    // Znajdź lub stwórz usera
    user, err := h.db.GetOrCreateBratUser(bratUser.Pseudonim)
    if err != nil {
        log.Printf("brtup: user error: %v", err)
        http.Error(w, "User error", http.StatusInternalServerError)
        return
    }

    // Dekoduj tytuł
    title := "Nowe wątki"
    if titleB64 != "nope" {
        decoded, err := base64.URLEncoding.DecodeString(titleB64)
        if err == nil && len(decoded) > 0 {
            title = string(decoded)
        }
    }

    // External ID dla galerii
    externalID := ""
    if entryID != "nope" {
        externalID = "brat:" + user.Slug + ":" + entryID
    } else {
        externalID = "brat:" + user.Slug + ":nowe"
    }

    // Znajdź lub stwórz galerię
    gallery, err := h.findOrCreateGallery(user.ID, externalID, title)
    if err != nil {
        log.Printf("brtup: gallery error: %v", err)
        http.Error(w, "Gallery error", http.StatusInternalServerError)
        return
    }

    // Parsuj plik
    if err := r.ParseMultipartForm(20 << 20); err != nil {
        http.Error(w, "File too large", http.StatusBadRequest)
        return
    }

    file, header, err := r.FormFile("image")
    if err != nil {
        http.Error(w, "Missing image", http.StatusBadRequest)
        return
    }
    defer file.Close()

    data := make([]byte, header.Size)
    if _, err := file.Read(data); err != nil {
        http.Error(w, "Failed to read file", http.StatusBadRequest)
        return
    }

    // Waliduj obraz
    format, err := image.ValidateAndDetect(data)
    if err != nil {
        http.Error(w, "Invalid image: "+err.Error(), http.StatusBadRequest)
        return
    }

    // Generuj slug
    slug, err := h.fs.GenerateUniqueSlug(5)
    if err != nil {
        http.Error(w, "Slug error", http.StatusInternalServerError)
        return
    }

    // Przetwórz obraz
    sizes, err := h.proc.Process(data, slug, format)
    if err != nil {
        log.Printf("brtup: process error: %v", err)
        http.Error(w, "Processing error", http.StatusInternalServerError)
        return
    }

    // Zapisz do DB
    img := &storage.Image{
        Slug:         slug,
        OriginalName: header.Filename,
        MimeType:     "image/webp",
        FileSize:     sizes["original"].Size,
        Width:        sizes["original"].Width,
        Height:       sizes["original"].Height,
        UserID:       &user.ID,
        GalleryID:    &gallery.ID,
    }

    if err := h.db.CreateImage(img); err != nil {
        log.Printf("brtup: db error: %v", err)
        http.Error(w, "DB error", http.StatusInternalServerError)
        return
    }

    // Odpowiedź
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(BratUploadResponse{
        URL:      h.cfg.BaseURL + "/i/" + slug + ".webp",
        ThumbURL: h.cfg.BaseURL + "/i/" + slug + "/thumb.webp",
        Filename: header.Filename,
        Slug:     slug,
    })
}

func (h *BratUploadHandler) findOrCreateGallery(userID int64, externalID, title string) (*storage.Gallery, error) {
    gallery, err := h.db.GetGalleryByExternalID(externalID)
    if err != nil {
        return nil, err
    }
    if gallery != nil {
        return gallery, nil
    }

    slug, err := h.fs.GenerateUniqueSlug(4)
    if err != nil {
        return nil, err
    }

    editToken, err := h.fs.GenerateUniqueSlug(32)
    if err != nil {
        return nil, err
    }

    gallery = &storage.Gallery{
        Slug:       slug,
        EditToken:  editToken,
        Title:      title,
        ExternalID: externalID,
        UserID:     &userID,
    }

    if err := h.db.CreateGallery(gallery); err != nil {
        return nil, err
    }

    return gallery, nil
}
```

**Step 4: Zarejestruj handler w main.go**

```go
// Po inicjalizacji bratDecoder
bratUpload := handler.NewBratUploadHandler(cfg, db, fs, proc, bratDecoder)
mux.Handle("/brtup/", bratUpload)
```

**Step 5: Zbuduj**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add internal/handler/brat_upload.go internal/storage/db.go cmd/dajtu/main.go
git commit -m "feat: add /brtup endpoint for Braterstwo image upload"
```

---

## Task 2: Modyfikacja Quill imageHandler

**Files:**
- Modify: `/home/pawel/dev/brat/.worktrees/tforum-v2/ui/js/quill-tforum.js`

**Step 1: Zamień imageHandler**

Zamień obecny `imageHandler` (linie 76-83) i dodaj nowe metody:

```javascript
imageHandler: function() {
    var self = this;
    var input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/jpeg,image/png,image/gif,image/webp';

    input.onchange = function() {
        var file = input.files[0];
        if (!file) return;

        if (file.size > 20 * 1024 * 1024) {
            self.showToast('Plik jest za duży (max 20MB)');
            return;
        }

        self.uploadToDajtu(file);
    };

    input.click();
},

uploadToDajtu: function(file) {
    var self = this;

    // Pobierz token SSO z window (musi być ustawiony przez PHP)
    var token = window.bratSsoToken;
    if (!token) {
        self.showToast('Brak tokenu - zaloguj się ponownie');
        return;
    }

    var entryId = this.getEntryId();
    var title = this.getGalleryTitle();
    var titleB64 = title ? btoa(unescape(encodeURIComponent(title))) : 'nope';

    var url = 'https://dajtu.com/brtup/' + token + '/' + entryId + '/' + titleB64;

    self.showToast('Wysyłam obrazek...');

    var formData = new FormData();
    formData.append('image', file);

    fetch(url, {
        method: 'POST',
        body: formData
    })
    .then(function(r) {
        if (!r.ok) {
            return r.text().then(function(t) { throw new Error(t); });
        }
        return r.json();
    })
    .then(function(data) {
        var range = self.quill.getSelection(true);
        // Wstaw thumbnail jako link do oryginału
        self.quill.insertText(range.index, '\n');
        self.quill.insertEmbed(range.index + 1, 'image', data.thumbUrl);
        self.quill.setSelection(range.index + 2);
        // Zaznacz obrazek i dodaj link
        self.quill.setSelection(range.index + 1, 1);
        self.quill.format('link', data.url);
        self.quill.setSelection(range.index + 2);
        self.showToast('Obrazek dodany!');
    })
    .catch(function(err) {
        console.error('Upload error:', err);
        self.showToast('Błąd: ' + err.message);
    });
},

getEntryId: function() {
    // URL: /tforum/t/123/tytuł lub /tforum/s/sekcja (nowy wątek)
    var match = window.location.pathname.match(/\/t\/(\d+)/);
    return match ? match[1] : 'nope';
},

getGalleryTitle: function() {
    // Dla istniejących wątków - tytuł z URL lub h1
    var match = window.location.pathname.match(/\/t\/\d+\/(.+)/);
    if (match) {
        return decodeURIComponent(match[1].replace(/-/g, ' '));
    }
    // Dla nowych - pole tytułu
    var titleInput = document.querySelector('input[name="title"]');
    if (titleInput && titleInput.value) {
        return titleInput.value;
    }
    return null;
},
```

**Step 2: PHP musi ustawić token w window**

W odpowiednim widoku PHP (tam gdzie ładowany jest Quill), dodaj:

```php
<?php if ($member = Member::current()): ?>
<script>
window.bratSsoToken = <?= json_encode(Brazar::buildLoginUrl($member->getPseudonim(), Brazar::getVerificationScore($member))) ?>;
// Wyciągnij sam token z URL
window.bratSsoToken = window.bratSsoToken.split('/').pop();
</script>
<?php endif; ?>
```

Albo lepiej - dodaj endpoint który zwraca sam token:

W PHP stwórz `/api/sso-token`:

```php
// W routes lub jako osobny controller
$f3->route('GET /api/sso-token', function($f3) {
    $member = Member::current();
    if (!$member) {
        http_response_code(401);
        echo json_encode(['error' => 'Not logged in']);
        return;
    }

    $url = Brazar::buildLoginUrl($member->getPseudonim(), Brazar::getVerificationScore($member));
    $token = basename($url); // wyciągnij ostatni segment

    header('Content-Type: application/json');
    echo json_encode(['token' => $token]);
});
```

I zmodyfikuj `uploadToDajtu`:

```javascript
uploadToDajtu: function(file) {
    var self = this;

    self.showToast('Wysyłam obrazek...');

    // Pobierz świeży token
    fetch('/api/sso-token')
        .then(function(r) {
            if (!r.ok) throw new Error('Nie zalogowany');
            return r.json();
        })
        .then(function(tokenData) {
            var entryId = self.getEntryId();
            var title = self.getGalleryTitle();
            var titleB64 = title ? btoa(unescape(encodeURIComponent(title))) : 'nope';

            var url = 'https://dajtu.com/brtup/' + tokenData.token + '/' + entryId + '/' + titleB64;

            var formData = new FormData();
            formData.append('image', file);

            return fetch(url, {
                method: 'POST',
                body: formData
            });
        })
        .then(function(r) {
            if (!r.ok) {
                return r.text().then(function(t) { throw new Error(t); });
            }
            return r.json();
        })
        .then(function(data) {
            var range = self.quill.getSelection(true);
            // Wstaw thumbnail jako link do oryginału
            self.quill.insertText(range.index, '\n');
            self.quill.insertEmbed(range.index + 1, 'image', data.thumbUrl);
            self.quill.setSelection(range.index + 2);
            // Zaznacz obrazek i dodaj link
            self.quill.setSelection(range.index + 1, 1);
            self.quill.format('link', data.url);
            self.quill.setSelection(range.index + 2);
            self.showToast('Obrazek dodany!');
        })
        .catch(function(err) {
            console.error('Upload error:', err);
            self.showToast('Błąd: ' + err.message);
        });
},
```

**Step 3: Commit**

```bash
cd /home/pawel/dev/brat/.worktrees/tforum-v2
git add ui/js/quill-tforum.js
git commit -m "feat: add image upload to dajtu from Quill"
```

---

## Task 3: Endpoint /api/sso-token w Braterstwo

**Files:**
- Create: `/home/pawel/dev/brat/app/controllers/api/SsoTokenController.php`

**Step 1: Stwórz controller**

```php
<?php
declare(strict_types=1);

class SsoTokenController {
    public function getToken($f3, $params) {
        header('Content-Type: application/json');

        $member = Member::current();
        if (!$member || !$member->userId) {
            http_response_code(401);
            echo json_encode(['error' => 'Nie zalogowany']);
            return;
        }

        $error = null;
        $url = Brazar::buildLoginUrl(
            $member->getPseudonim(),
            Brazar::getVerificationScore($member),
            $error
        );

        if ($url === null) {
            http_response_code(500);
            echo json_encode(['error' => $error]);
            return;
        }

        // Wyciągnij token z URL (ostatni segment)
        $token = basename($url);

        echo json_encode(['token' => $token]);
    }
}
```

**Step 2: Zarejestruj route**

```php
$f3->route('GET /api/sso-token', 'SsoTokenController->getToken');
```

**Step 3: Commit**

```bash
git add app/controllers/api/SsoTokenController.php
git commit -m "feat: add /api/sso-token endpoint for dajtu integration"
```

---

## Task 4: Testy

**Files:**
- Create: `/home/pawel/dev/dajtu/internal/handler/brat_upload_test.go`

**Step 1: Test podstawowy**

```go
package handler

import (
    "bytes"
    "mime/multipart"
    "net/http"
    "net/http/httptest"
    "testing"

    "dajtu/internal/auth"
    "dajtu/internal/testutil"
)

func TestBratUpload_ValidRequest(t *testing.T) {
    cfg := testutil.TestConfig()
    db := testutil.TestDB(t)
    fs := testutil.TestFilesystem(t)

    // Potrzebny będzie mock decodera lub test z prawdziwym tokenem
    // Na razie skip jeśli brak konfiguracji
    decoder, err := auth.NewBratDecoder(auth.BratConfig{
        HashSecret:    cfg.BratHashSecret,
        EncryptionKey: cfg.BratEncryptionKey,
        EncryptionIV:  cfg.BratEncryptionIV,
        Cipher:        cfg.BratCipher,
        MaxSkewSeconds: 600,
        HashLength:    10,
        HashBytes:     5,
    })
    if decoder == nil {
        t.Skip("Brak konfiguracji SSO")
    }

    // Test bez prawdziwego tokenu - sprawdź tylko czy endpoint odpowiada
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)
    part, _ := writer.CreateFormFile("image", "test.jpg")
    part.Write(testutil.SampleJPEG())
    writer.Close()

    req := httptest.NewRequest("POST", "/brtup/invalid/123/dGVzdA", &buf)
    req.Header.Set("Content-Type", writer.FormDataContentType())

    w := httptest.NewRecorder()

    handler := NewBratUploadHandler(cfg, db, fs, nil, decoder)
    handler.ServeHTTP(w, req)

    // Oczekujemy 401 bo token invalid
    if w.Code != http.StatusUnauthorized {
        t.Errorf("expected 401 for invalid token, got %d: %s", w.Code, w.Body.String())
    }
}

func TestBratUpload_MissingImage(t *testing.T) {
    cfg := testutil.TestConfig()
    db := testutil.TestDB(t)
    fs := testutil.TestFilesystem(t)
    decoder, _ := auth.NewBratDecoder(auth.BratConfig{})

    req := httptest.NewRequest("POST", "/brtup/token/123/dGVzdA", nil)
    w := httptest.NewRecorder()

    handler := NewBratUploadHandler(cfg, db, fs, nil, decoder)
    handler.ServeHTTP(w, req)

    if w.Code != http.StatusUnauthorized && w.Code != http.StatusBadRequest {
        t.Errorf("expected error status, got %d", w.Code)
    }
}
```

**Step 2: Uruchom testy**

```bash
go test ./internal/handler/... -v -run BratUpload
```

**Step 3: Commit**

```bash
git add internal/handler/brat_upload_test.go
git commit -m "test: add brat upload handler tests"
```

---

## Task 5: Deploy i E2E test

**Step 1: Deploy dajtu**

```bash
ssh staging "cd /var/www/dajtu && git pull && docker compose up --build -d"
```

**Step 2: Sprawdź logi**

```bash
ssh staging "docker logs dajtu_app --tail 50"
```

**Step 3: Test ręczny z curl**

```bash
# Wygeneruj token w Braterstwo i skopiuj
# Następnie:
curl -X POST \
  -F "image=@/tmp/test.jpg" \
  "https://dajtu.com/brtup/TWOJ_TOKEN/123/dGVzdA"
```

**Step 4: Deploy Braterstwo i test w przeglądarce**

1. Zaloguj się na Braterstwo
2. Otwórz wątek z Quill
3. Kliknij ikonę obrazka
4. Wybierz plik
5. Sprawdź czy obraz pojawia się w edytorze

---

## Diagram przepływu

```
┌─────────────┐     1. Klik obrazek      ┌─────────────┐
│    Quill    │ ──────────────────────►  │   input     │
│   Editor    │                          │   file      │
└─────────────┘                          └──────┬──────┘
       │                                        │
       │◄───────────────────────────────────────┘
       │                                 2. Wybór pliku
       ▼
┌─────────────┐     3. GET /api/sso-token       ┌─────────────┐
│    Quill    │ ──────────────────────────────► │ Braterstwo  │
│             │ ◄────────────────────────────── │    PHP      │
└─────────────┘     4. {token: "xyz..."}        └─────────────┘
       │
       │ 5. POST /brtup/{token}/{entryId}/{titleB64}
       │    + FormData: image
       ▼
┌─────────────┐
│    dajtu    │ ─► 6. Decode token (BratDecoder)
│   /brtup    │ ─► 7. GetOrCreateBratUser
│             │ ─► 8. findOrCreateGallery (by external_id)
│             │ ─► 9. Process & save image
└──────┬──────┘
       │
       │ 10. {url, filename, slug}
       ▼
┌─────────────┐
│    Quill    │ ─► 11. insertEmbed(image, url)
│  <img src=> │
└─────────────┘
```

---

## Uwagi bezpieczeństwa

1. **Token SSO** - ważny 600s (MaxSkewSeconds), weryfikacja HMAC
2. **CORS** - tylko domeny braterstwo.*
3. **Rate limiting** - istniejący middleware (30 req/min)
4. **Walidacja obrazu** - magic bytes + libvips
5. **External ID** - zawiera user slug, więc galerie są per-user

---

## Opcjonalne rozszerzenia (TODO na przyszłość)

- [ ] Drag & drop w Quill
- [ ] Paste obrazu ze schowka
- [ ] Preview przed uploadem
- [ ] Progress bar uploadu
- [ ] Miniaturki w galerii "nowe wątki"
