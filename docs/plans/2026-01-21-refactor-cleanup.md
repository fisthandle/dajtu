# Refaktoryzacja dajtu - cleanup i przeglądarka zdjęć

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wyczyścić kod z duplikatów, naprawić błędy, dodać przeglądarkę zdjęć z lazy loading.

**Architecture:** Wydzielenie wspólnych helperów, centralizacja logiki, lightbox dla galerii.

**Tech Stack:** Go, vanilla JS (bez frameworków - prostota)

---

## Task 1: Napraw CORS - właściwa domena

**Files:**
- Modify: `/home/pawel/dev/dajtu/internal/handler/brat_upload.go`
- Modify: `/home/pawel/dev/dajtu/internal/handler/brat_upload_test.go`

**Problem:** Hardcoded `braterstwo.pl` i `braterstwo.com` - te domeny nie istnieją. Właściwa to `braterstwo.eu`.

**Step 1:** Zmień CORS w `brat_upload.go:40`:

```go
// Było:
if strings.Contains(origin, "braterstwo.pl") || strings.Contains(origin, "braterstwo.com") || strings.Contains(origin, "localhost") {

// Ma być:
if strings.Contains(origin, "braterstwo.eu") || strings.Contains(origin, "localhost") {
```

**Step 2:** Napraw testy w `brat_upload_test.go` - zamień wszystkie `braterstwo.pl` i `braterstwo.com` na `braterstwo.eu`.

**Step 3:** Commit

```bash
git add internal/handler/brat_upload.go internal/handler/brat_upload_test.go
git commit -m "fix: correct CORS domain to braterstwo.eu"
```

---

## Task 2: Wydziel wspólne helpery

**Files:**
- Create: `/home/pawel/dev/dajtu/internal/handler/helpers.go`
- Modify: `/home/pawel/dev/dajtu/internal/handler/upload.go`
- Modify: `/home/pawel/dev/dajtu/internal/handler/gallery.go`
- Modify: `/home/pawel/dev/dajtu/internal/handler/brat_upload.go`
- Modify: `/home/pawel/dev/dajtu/internal/storage/db.go`

**Step 1:** Stwórz `helpers.go`:

```go
package handler

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"

    "dajtu/internal/config"
    "dajtu/internal/storage"
)

// jsonError wysyła błąd JSON
func jsonError(w http.ResponseWriter, message string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// jsonSuccess wysyła sukces JSON
func jsonSuccess(w http.ResponseWriter, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}

// getBaseURL zwraca base URL z konfiguracji lub z requestu
func getBaseURL(cfg *config.Config, r *http.Request) string {
    if cfg.BaseURL != "" {
        return cfg.BaseURL
    }
    scheme := "http"
    if r.TLS != nil {
        scheme = "https"
    }
    return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// buildImageURL buduje URL do obrazka
func buildImageURL(baseURL, slug, size string) string {
    if size == "" || size == "original" {
        return fmt.Sprintf("%s/i/%s.webp", baseURL, slug)
    }
    return fmt.Sprintf("%s/i/%s/%s.webp", baseURL, slug, size)
}

// generateEditToken generuje 32-znakowy token
func generateEditToken() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        // Fallback - nie powinno się zdarzyć
        return storage.GenerateSlug(32)
    }
    return hex.EncodeToString(b)
}
```

**Step 2:** Przenieś `GenerateUniqueSlug` do `storage/db.go`:

```go
// GenerateUniqueSlug generuje unikalny slug dla tabeli
func (db *DB) GenerateUniqueSlug(table string, length int) string {
    // Whitelist tabel dla bezpieczeństwa
    validTables := map[string]bool{"images": true, "galleries": true, "users": true}
    if !validTables[table] {
        return GenerateSlug(length)
    }

    candidates := make([]string, 20)
    for i := range candidates {
        candidates[i] = GenerateSlug(length)
    }

    for _, slug := range candidates {
        exists, err := db.SlugExists(table, slug)
        if err == nil && !exists {
            return slug
        }
    }
    // Rekurencja - bardzo mało prawdopodobne
    return db.GenerateUniqueSlug(table, length)
}
```

**Step 3:** Usuń zduplikowane funkcje z `upload.go`, `gallery.go`, `brat_upload.go` i użyj nowych helperów.

**Step 4:** Commit

```bash
git add internal/handler/helpers.go internal/handler/*.go internal/storage/db.go
git commit -m "refactor: extract common helpers, remove duplicates"
```

---

## Task 3: Napraw błędy krytyczne

**Files:**
- Modify: `/home/pawel/dev/dajtu/cmd/dajtu/main.go`
- Modify: `/home/pawel/dev/dajtu/internal/handler/brat_upload.go`
- Modify: `/home/pawel/dev/dajtu/internal/storage/db.go`

**Step 1:** Walidacja slug przeciw path traversal w `main.go`:

```go
// Dodaj funkcję walidacji
func isValidSlug(s string) bool {
    if len(s) < 2 || len(s) > 10 {
        return false
    }
    for _, c := range s {
        if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
            return false
        }
    }
    return true
}

// Użyj przed http.ServeFile:
if !isValidSlug(slug) {
    http.NotFound(w, r)
    return
}
```

**Step 2:** Napraw FileSize w `brat_upload.go` - sumuj rozmiary wszystkich wersji:

```go
var totalSize int64
if h.cfg.KeepOriginalFormat {
    size, err := h.fs.SaveOriginal(slug, "original", data, string(format))
    if err == nil {
        totalSize += size
    }
}

for _, res := range results {
    if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
        // ...
    }
    totalSize += int64(len(res.Data))
}

img := &storage.Image{
    // ...
    FileSize: totalSize,
}
```

**Step 3:** Obsłuż błąd rand.Read w `storage/db.go`:

```go
func GenerateSlug(length int) string {
    b := make([]byte, length/2+1)
    if _, err := rand.Read(b); err != nil {
        // Fallback na time-based
        return fmt.Sprintf("%x", time.Now().UnixNano())[:length]
    }
    return hex.EncodeToString(b)[:length]
}
```

**Step 4:** Commit

```bash
git add cmd/dajtu/main.go internal/handler/brat_upload.go internal/storage/db.go
git commit -m "fix: path traversal, FileSize calculation, rand.Read error handling"
```

---

## Task 4: Przeglądarka zdjęć - Lightbox

**Files:**
- Create: `/home/pawel/dev/dajtu/internal/handler/templates/lightbox.css`
- Create: `/home/pawel/dev/dajtu/internal/handler/templates/lightbox.js`
- Modify: `/home/pawel/dev/dajtu/internal/handler/templates/gallery.html`

**Wymagania:**
- Strzałki lewo/prawo (klawiatura + klik)
- Lazy loading obrazków
- Obsługa setek obrazków (virtual scroll lub pagination)
- Minimalistyczny design
- Zero zależności (vanilla JS)

**Step 1:** Stwórz `lightbox.css`:

```css
/* Galeria z lazy loading */
.gallery-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
    gap: 4px;
}

.gallery-thumb {
    aspect-ratio: 1;
    object-fit: cover;
    cursor: pointer;
    background: #1a1a1a;
    transition: transform 0.2s;
}

.gallery-thumb:hover {
    transform: scale(1.05);
}

.gallery-thumb[data-src] {
    opacity: 0;
}

.gallery-thumb.loaded {
    opacity: 1;
    transition: opacity 0.3s;
}

/* Lightbox */
.lightbox {
    display: none;
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.95);
    z-index: 1000;
    align-items: center;
    justify-content: center;
}

.lightbox.active {
    display: flex;
}

.lightbox-img {
    max-width: 90vw;
    max-height: 90vh;
    object-fit: contain;
}

.lightbox-nav {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    background: rgba(255,255,255,0.1);
    border: none;
    color: white;
    font-size: 3rem;
    padding: 1rem;
    cursor: pointer;
    user-select: none;
}

.lightbox-nav:hover {
    background: rgba(255,255,255,0.2);
}

.lightbox-prev { left: 1rem; }
.lightbox-next { right: 1rem; }

.lightbox-close {
    position: absolute;
    top: 1rem;
    right: 1rem;
    background: none;
    border: none;
    color: white;
    font-size: 2rem;
    cursor: pointer;
}

.lightbox-counter {
    position: absolute;
    bottom: 1rem;
    left: 50%;
    transform: translateX(-50%);
    color: white;
    font-size: 0.9rem;
}

/* Preloader */
.lightbox-loading {
    position: absolute;
    color: white;
}
```

**Step 2:** Stwórz `lightbox.js`:

```javascript
(function() {
    'use strict';

    // Lazy loading z IntersectionObserver
    var observer = new IntersectionObserver(function(entries) {
        entries.forEach(function(entry) {
            if (entry.isIntersecting) {
                var img = entry.target;
                img.src = img.dataset.src;
                img.onload = function() { img.classList.add('loaded'); };
                observer.unobserve(img);
            }
        });
    }, { rootMargin: '100px' });

    document.querySelectorAll('.gallery-thumb[data-src]').forEach(function(img) {
        observer.observe(img);
    });

    // Lightbox
    var lightbox = document.getElementById('lightbox');
    if (!lightbox) return;

    var images = Array.from(document.querySelectorAll('.gallery-thumb'));
    var currentIndex = 0;
    var lightboxImg = lightbox.querySelector('.lightbox-img');
    var counter = lightbox.querySelector('.lightbox-counter');

    function show(index) {
        if (index < 0) index = images.length - 1;
        if (index >= images.length) index = 0;
        currentIndex = index;

        var thumb = images[index];
        // Pobierz URL pełnego obrazu (bez /thumb)
        var fullUrl = thumb.dataset.full || thumb.src.replace('/thumb.webp', '.webp');

        lightboxImg.style.opacity = '0';
        lightboxImg.onload = function() { lightboxImg.style.opacity = '1'; };
        lightboxImg.src = fullUrl;

        if (counter) {
            counter.textContent = (index + 1) + ' / ' + images.length;
        }

        lightbox.classList.add('active');
        document.body.style.overflow = 'hidden';
    }

    function hide() {
        lightbox.classList.remove('active');
        document.body.style.overflow = '';
    }

    function next() { show(currentIndex + 1); }
    function prev() { show(currentIndex - 1); }

    // Event listeners
    images.forEach(function(img, i) {
        img.addEventListener('click', function() { show(i); });
    });

    lightbox.querySelector('.lightbox-close').addEventListener('click', hide);
    lightbox.querySelector('.lightbox-prev').addEventListener('click', prev);
    lightbox.querySelector('.lightbox-next').addEventListener('click', next);

    lightbox.addEventListener('click', function(e) {
        if (e.target === lightbox) hide();
    });

    document.addEventListener('keydown', function(e) {
        if (!lightbox.classList.contains('active')) return;
        if (e.key === 'Escape') hide();
        if (e.key === 'ArrowLeft') prev();
        if (e.key === 'ArrowRight') next();
    });

    // Preload sąsiednich obrazków
    function preloadAdjacent() {
        [-1, 1].forEach(function(offset) {
            var i = currentIndex + offset;
            if (i >= 0 && i < images.length) {
                var img = new Image();
                img.src = images[i].dataset.full || images[i].src.replace('/thumb.webp', '.webp');
            }
        });
    }

    lightboxImg.addEventListener('load', preloadAdjacent);
})();
```

**Step 3:** Zmodyfikuj `gallery.html`:

```html
<!-- Dodaj CSS -->
<style>{{template "lightbox.css"}}</style>

<!-- Galeria z lazy loading -->
<div class="gallery-grid">
    {{range .Images}}
    <img
        class="gallery-thumb"
        data-src="{{$.BaseURL}}/i/{{.Slug}}/thumb.webp"
        data-full="{{$.BaseURL}}/i/{{.Slug}}.webp"
        alt="{{.OriginalName}}"
        loading="lazy"
    >
    {{end}}
</div>

<!-- Lightbox -->
<div id="lightbox" class="lightbox">
    <button class="lightbox-close">&times;</button>
    <button class="lightbox-nav lightbox-prev">&#8249;</button>
    <img class="lightbox-img" src="" alt="">
    <button class="lightbox-nav lightbox-next">&#8250;</button>
    <div class="lightbox-counter"></div>
</div>

<script>{{template "lightbox.js"}}</script>
```

**Step 4:** Zaktualizuj embed w `gallery.go`:

```go
//go:embed templates/*
var templatesFS embed.FS
```

**Step 5:** Commit

```bash
git add internal/handler/templates/
git commit -m "feat: add lightbox gallery viewer with lazy loading"
```

---

## Task 5: Dodaj paginację dla dużych galerii

**Files:**
- Modify: `/home/pawel/dev/dajtu/internal/storage/db.go`
- Modify: `/home/pawel/dev/dajtu/internal/handler/gallery.go`

**Step 1:** Dodaj metodę z paginacją w `db.go`:

```go
func (db *DB) GetGalleryImagesPaginated(galleryID int64, limit, offset int) ([]*Image, int, error) {
    // Pobierz total count
    var total int
    err := db.conn.QueryRow(`SELECT COUNT(*) FROM images WHERE gallery_id = ?`, galleryID).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    // Pobierz stronę
    rows, err := db.conn.Query(`
        SELECT id, slug, original_name, mime_type, file_size, width, height, created_at
        FROM images WHERE gallery_id = ?
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `, galleryID, limit, offset)
    // ... reszta jak w GetGalleryImages
}
```

**Step 2:** Użyj w handlerze z query param `?page=N`:

```go
page, _ := strconv.Atoi(r.URL.Query().Get("page"))
if page < 1 { page = 1 }
limit := 100
offset := (page - 1) * limit

images, total, err := h.db.GetGalleryImagesPaginated(gallery.ID, limit, offset)
totalPages := (total + limit - 1) / limit
```

**Step 3:** Commit

```bash
git add internal/storage/db.go internal/handler/gallery.go
git commit -m "feat: add pagination for large galleries"
```

---

## Task 6: Testy i cleanup

**Step 1:** Uruchom wszystkie testy:

```bash
go test ./... -v
```

**Step 2:** Sprawdź build:

```bash
go build ./...
```

**Step 3:** Usuń nieużywane pliki/kod

**Step 4:** Final commit

```bash
git add -A
git commit -m "chore: cleanup and verify all tests pass"
```

---

## Diagram zmian

```
PRZED:                          PO:

upload.go ─┐                   helpers.go (wspólne)
gallery.go ├─ generateSlug()    ├─ jsonError()
brat_up.go ┘   (3x)             ├─ jsonSuccess()
                                ├─ getBaseURL()
                                ├─ buildImageURL()
                                └─ generateEditToken()

                               storage/db.go
                                └─ GenerateUniqueSlug()

gallery.html (prosty)          gallery.html + lightbox
                                ├─ lazy loading (IntersectionObserver)
                                ├─ strzałki ←→
                                ├─ keyboard nav
                                └─ preload sąsiednich
```

---

## Priorytety

1. **Task 1** - CORS fix (5 min) ← najpierw to
2. **Task 3** - błędy krytyczne (15 min)
3. **Task 2** - helpery (20 min)
4. **Task 4** - lightbox (30 min)
5. **Task 5** - paginacja (15 min)
6. **Task 6** - testy (10 min)
