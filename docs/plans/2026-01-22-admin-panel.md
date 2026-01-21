# Admin Panel Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Panel administracyjny dla wybranych użytkowników Braterstwa z możliwością przeglądania i kasowania zdjęć/galerii/kont oraz statystykami.

**Architecture:** Nowy middleware `AdminOnly` sprawdza czy zalogowany user jest na liście adminów (env var ADMIN_NICKS). Nowe handlery w `internal/handler/admin.go` obsługują endpointy `/admin/*`. Widok oparty na szablonach HTML (jak index.tmpl). Licznik pobrań w tabeli images (nowa kolumna `downloads`). Istniejące pole `accessed_at` aktualizowane przy każdym dostępie - cleanup usuwa najdawniej używane pliki.

**Tech Stack:** Go 1.22, SQLite, HTML templates, htmx (opcjonalnie dla delete bez reload)

---

## Task 1: Migracja DB - kolumna downloads

**Files:**
- Modify: `internal/storage/db.go:50-60` (schema)

**Step 1: Dodaj kolumnę downloads do schema**

W funkcji `ensureSchema`, dodaj migrację:

```go
// W ensureSchema(), po istniejących migracjach:
_, err = db.Exec(`ALTER TABLE images ADD COLUMN downloads INTEGER NOT NULL DEFAULT 0`)
if err != nil && !strings.Contains(err.Error(), "duplicate column") {
    return nil, fmt.Errorf("migrate downloads: %w", err)
}
```

**Step 2: Zaktualizuj struct Image**

W `internal/storage/db.go`, dodaj pole do struct:

```go
type Image struct {
    // ... istniejące pola
    Downloads   int64  // po AccessedAt
}
```

**Step 3: Zaktualizuj zapytania SELECT dla Image**

Znajdź wszystkie `SELECT ... FROM images` i dodaj `downloads` do listy kolumn.

**Step 4: Uruchom aplikację i sprawdź migrację**

```bash
cd /home/pawel/dev/dajtu && go run ./cmd/dajtu
# Sprawdź logi - nie powinno być błędów
# Ctrl+C
```

**Step 5: Commit**

```bash
git add internal/storage/db.go
git commit -m "feat: add downloads counter column to images table"
```

---

## Task 2: Aktualizacja statystyk przy pobraniu

**Files:**
- Modify: `internal/storage/db.go` (nowa metoda)
- Modify: `cmd/dajtu/main.go:98-134` (handler /i/)

**Step 1: Dodaj metodę IncrementDownloads**

W `internal/storage/db.go`:

```go
func (s *DB) IncrementDownloads(slug string) error {
    _, err := s.db.Exec(`UPDATE images SET downloads = downloads + 1 WHERE slug = ?`, slug)
    return err
}
```

**Step 2: Wywołaj oba updaty w handlerze /i/**

W `cmd/dajtu/main.go`, w handlerze `/i/`:

```go
// Po walidacji sluga, przed http.ServeFile:
go func() {
    _ = db.TouchImageBySlug(slug)    // aktualizuj accessed_at (już istnieje!)
    _ = db.IncrementDownloads(slug)  // inkrementuj licznik pobrań
}()
```

**Uwaga:** `TouchImageBySlug` już istnieje w `db.go:199` - tylko nie było nigdzie wywoływane!

**Step 3: Test manualny**

```bash
go run ./cmd/dajtu &
curl -I http://localhost:8080/i/abc12/thumb
sqlite3 data/dajtu.db "SELECT slug, downloads, accessed_at FROM images WHERE slug='abc12'"
# downloads > 0, accessed_at = świeży timestamp
```

**Step 4: Commit**

```bash
git add internal/storage/db.go cmd/dajtu/main.go
git commit -m "feat: update accessed_at and downloads counter on image view"
```

---

## Task 3: Zmiana logiki cleanup - usuwaj najdawniej używane

**Files:**
- Modify: `internal/storage/db.go` (GetOldestImages)
- Modify: `internal/cleanup/daemon.go` (opcjonalnie - logowanie)

**Step 1: Zmień sortowanie w GetOldestImages**

W `internal/storage/db.go`, znajdź funkcję `GetOldestImages` i zmień `ORDER BY created_at` na `ORDER BY accessed_at`:

```go
func (db *DB) GetOldestImages(limit int) ([]*Image, error) {
    rows, err := db.conn.Query(`
        SELECT id, slug, file_size, created_at, accessed_at
        FROM images
        ORDER BY accessed_at ASC  -- było: created_at ASC
        LIMIT ?`, limit)
    // ...
}
```

**Step 2: Test manualny**

```bash
# Sprawdź że cleanup bierze najdawniej UŻYWANE, nie najstarsze
sqlite3 data/dajtu.db "SELECT slug, created_at, accessed_at FROM images ORDER BY accessed_at ASC LIMIT 5"
```

**Step 3: Commit**

```bash
git add internal/storage/db.go
git commit -m "feat: cleanup removes least recently accessed images first"
```

---

## Task 4: Konfiguracja adminów

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Dodaj pole AdminNicks do Config**

```go
type Config struct {
    // ... istniejące pola
    AdminNicks []string // lista nicków z prawami admina
}
```

**Step 2: Parsuj ADMIN_NICKS z env**

W funkcji `Load()`:

```go
adminNicks := []string{"KS Amator", "gruby wonsz"} // domyślne
if v := os.Getenv("ADMIN_NICKS"); v != "" {
    adminNicks = strings.Split(v, ",")
    for i := range adminNicks {
        adminNicks[i] = strings.TrimSpace(adminNicks[i])
    }
}
cfg.AdminNicks = adminNicks
```

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add ADMIN_NICKS config for admin panel access"
```

---

## Task 5: Middleware AdminOnly

**Files:**
- Create: `internal/middleware/admin.go`

**Step 1: Utwórz middleware**

```go
package middleware

import (
    "net/http"
    "slices"
)

type AdminMiddleware struct {
    adminNicks []string
}

func NewAdminMiddleware(adminNicks []string) *AdminMiddleware {
    return &AdminMiddleware{adminNicks: adminNicks}
}

func (m *AdminMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := GetUser(r)
        if user == nil {
            http.Redirect(w, r, "/", http.StatusSeeOther)
            return
        }
        if !slices.Contains(m.adminNicks, user.DisplayName) {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Step 2: Sprawdź kompilację**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/middleware/admin.go
git commit -m "feat: add AdminOnly middleware for admin panel"
```

---

## Task 6: Handler admina - dashboard ze statystykami

**Files:**
- Create: `internal/handler/admin.go`
- Modify: `internal/storage/db.go` (metody statystyk)

**Step 1: Dodaj metody statystyk do DB**

W `internal/storage/db.go`:

```go
type Stats struct {
    TotalImages    int64
    TotalGalleries int64
    TotalUsers     int64
    DiskUsageBytes int64
}

func (s *DB) GetStats() (*Stats, error) {
    var stats Stats

    row := s.db.QueryRow(`SELECT COUNT(*) FROM images`)
    row.Scan(&stats.TotalImages)

    row = s.db.QueryRow(`SELECT COUNT(*) FROM galleries`)
    row.Scan(&stats.TotalGalleries)

    row = s.db.QueryRow(`SELECT COUNT(*) FROM users`)
    row.Scan(&stats.TotalUsers)

    row = s.db.QueryRow(`SELECT COALESCE(SUM(file_size), 0) FROM images`)
    row.Scan(&stats.DiskUsageBytes)

    return &stats, nil
}
```

**Step 2: Utwórz handler admina**

```go
package handler

import (
    "html/template"
    "net/http"

    "dajtu/internal/storage"
)

type AdminHandler struct {
    db   *storage.DB
    tmpl *template.Template
}

func NewAdminHandler(db *storage.DB) *AdminHandler {
    return &AdminHandler{
        db:   db,
        tmpl: template.Must(template.ParseGlob("templates/admin/*.html")),
    }
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
    stats, err := h.db.GetStats()
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    h.tmpl.ExecuteTemplate(w, "dashboard.html", stats)
}
```

**Step 3: Commit**

```bash
git add internal/handler/admin.go internal/storage/db.go
git commit -m "feat: add admin handler with stats dashboard"
```

---

## Task 7: Szablony HTML admina

**Files:**
- Create: `templates/admin/layout.html`
- Create: `templates/admin/dashboard.html`

**Step 1: Layout bazowy**

```html
{{define "layout"}}
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Admin - dajtu</title>
    <style>
        body { font-family: system-ui; max-width: 1200px; margin: 0 auto; padding: 20px; }
        nav { margin-bottom: 20px; }
        nav a { margin-right: 15px; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        .stat { display: inline-block; padding: 20px; margin: 10px; background: #f5f5f5; border-radius: 8px; }
        .stat-value { font-size: 2em; font-weight: bold; }
        .btn-delete { background: #dc3545; color: white; border: none; padding: 5px 10px; cursor: pointer; }
    </style>
</head>
<body>
    <nav>
        <a href="/admin">Dashboard</a>
        <a href="/admin/users">Konta</a>
        <a href="/admin/galleries">Galerie</a>
        <a href="/admin/images">Zdjęcia</a>
    </nav>
    {{template "content" .}}
</body>
</html>
{{end}}
```

**Step 2: Dashboard**

```html
{{define "content"}}
<h1>Dashboard</h1>
<div>
    <div class="stat">
        <div class="stat-value">{{.TotalImages}}</div>
        <div>Zdjęć</div>
    </div>
    <div class="stat">
        <div class="stat-value">{{.TotalGalleries}}</div>
        <div>Galerii</div>
    </div>
    <div class="stat">
        <div class="stat-value">{{.TotalUsers}}</div>
        <div>Użytkowników</div>
    </div>
    <div class="stat">
        <div class="stat-value">{{printf "%.2f" (divf .DiskUsageBytes 1073741824)}} GB</div>
        <div>Zajęte miejsce</div>
    </div>
</div>
{{end}}
```

**Step 3: Commit**

```bash
git add templates/admin/
git commit -m "feat: add admin HTML templates"
```

---

## Task 8: Lista użytkowników

**Files:**
- Modify: `internal/storage/db.go` (ListUsers)
- Modify: `internal/handler/admin.go`
- Create: `templates/admin/users.html`

**Step 1: Dodaj ListUsers do DB**

```go
func (s *DB) ListUsers(limit, offset int) ([]*User, error) {
    rows, err := s.db.Query(`
        SELECT id, slug, display_name, created_at, updated_at
        FROM users
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []*User
    for rows.Next() {
        u := &User{}
        rows.Scan(&u.ID, &u.Slug, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
        users = append(users, u)
    }
    return users, nil
}
```

**Step 2: Handler Users**

W `internal/handler/admin.go`:

```go
func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
    users, _ := h.db.ListUsers(100, 0)
    h.tmpl.ExecuteTemplate(w, "users.html", users)
}
```

**Step 3: Szablon users.html**

```html
{{define "content"}}
<h1>Użytkownicy</h1>
<table>
    <tr><th>ID</th><th>Nick</th><th>Slug</th><th>Utworzony</th></tr>
    {{range .}}
    <tr>
        <td>{{.ID}}</td>
        <td>{{.DisplayName}}</td>
        <td>{{.Slug}}</td>
        <td>{{.CreatedAt}}</td>
    </tr>
    {{end}}
</table>
{{end}}
```

**Step 4: Commit**

```bash
git add internal/storage/db.go internal/handler/admin.go templates/admin/users.html
git commit -m "feat: add users list in admin panel"
```

---

## Task 9: Lista galerii z możliwością kasowania

**Files:**
- Modify: `internal/storage/db.go` (ListGalleries, DeleteGallery)
- Modify: `internal/handler/admin.go`
- Create: `templates/admin/galleries.html`

**Step 1: Dodaj metody do DB**

```go
type GalleryWithCount struct {
    Gallery
    ImageCount int
    OwnerName  string
}

func (s *DB) ListGalleriesAdmin(limit, offset int) ([]*GalleryWithCount, error) {
    rows, err := s.db.Query(`
        SELECT g.id, g.slug, g.title, g.user_id, g.created_at,
               COUNT(i.id) as image_count,
               COALESCE(u.display_name, '') as owner_name
        FROM galleries g
        LEFT JOIN images i ON i.gallery_id = g.id
        LEFT JOIN users u ON u.id = g.user_id
        GROUP BY g.id
        ORDER BY g.created_at DESC
        LIMIT ? OFFSET ?
    `, limit, offset)
    // ... scan logic
}

func (s *DB) DeleteGallery(id int64) error {
    // CASCADE usuwa też obrazki z tabeli, ale trzeba usunąć pliki
    _, err := s.db.Exec(`DELETE FROM galleries WHERE id = ?`, id)
    return err
}
```

**Step 2: Handler Galleries + DeleteGallery**

```go
func (h *AdminHandler) Galleries(w http.ResponseWriter, r *http.Request) {
    galleries, _ := h.db.ListGalleriesAdmin(100, 0)
    h.tmpl.ExecuteTemplate(w, "galleries.html", galleries)
}

func (h *AdminHandler) DeleteGallery(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)

    // Pobierz slugi obrazków do usunięcia z dysku
    images, _ := h.db.GetImagesByGallery(id)

    // Usuń z DB (CASCADE)
    h.db.DeleteGallery(id)

    // Usuń pliki
    for _, img := range images {
        h.fs.DeleteImage(img.Slug)
    }

    http.Redirect(w, r, "/admin/galleries", http.StatusSeeOther)
}
```

**Step 3: Szablon galleries.html**

```html
{{define "content"}}
<h1>Galerie</h1>
<table>
    <tr><th>ID</th><th>Tytuł</th><th>Właściciel</th><th>Zdjęć</th><th>Akcje</th></tr>
    {{range .}}
    <tr>
        <td>{{.ID}}</td>
        <td><a href="/g/{{.Slug}}">{{if .Title}}{{.Title}}{{else}}(bez tytułu){{end}}</a></td>
        <td>{{.OwnerName}}</td>
        <td>{{.ImageCount}}</td>
        <td>
            <form method="POST" action="/admin/galleries/{{.ID}}/delete" onsubmit="return confirm('Usunąć galerię ze wszystkimi zdjęciami?')">
                <button class="btn-delete">Usuń</button>
            </form>
        </td>
    </tr>
    {{end}}
</table>
{{end}}
```

**Step 4: Commit**

```bash
git add internal/storage/db.go internal/handler/admin.go templates/admin/galleries.html
git commit -m "feat: add galleries list with delete in admin panel"
```

---

## Task 10: Lista zdjęć z możliwością kasowania

**Files:**
- Modify: `internal/storage/db.go` (ListImagesAdmin)
- Modify: `internal/handler/admin.go`
- Create: `templates/admin/images.html`

**Step 1: Dodaj ListImagesAdmin**

```go
type ImageAdmin struct {
    Image
    OwnerName   string
    GallerySlug string
}

func (s *DB) ListImagesAdmin(limit, offset int) ([]*ImageAdmin, error) {
    rows, err := s.db.Query(`
        SELECT i.id, i.slug, i.original_name, i.file_size, i.downloads, i.created_at,
               COALESCE(u.display_name, '') as owner_name,
               COALESCE(g.slug, '') as gallery_slug
        FROM images i
        LEFT JOIN users u ON u.id = i.user_id
        LEFT JOIN galleries g ON g.id = i.gallery_id
        ORDER BY i.created_at DESC
        LIMIT ? OFFSET ?
    `, limit, offset)
    // ... scan logic
}
```

**Step 2: Handler Images + DeleteImage**

```go
func (h *AdminHandler) Images(w http.ResponseWriter, r *http.Request) {
    images, _ := h.db.ListImagesAdmin(100, 0)
    h.tmpl.ExecuteTemplate(w, "images.html", images)
}

func (h *AdminHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)

    img, _ := h.db.GetImageByID(id)
    if img != nil {
        h.db.DeleteImage(id)
        h.fs.DeleteImage(img.Slug)
    }

    http.Redirect(w, r, "/admin/images", http.StatusSeeOther)
}
```

**Step 3: Szablon images.html**

```html
{{define "content"}}
<h1>Zdjęcia</h1>
<table>
    <tr><th>Podgląd</th><th>Nazwa</th><th>Właściciel</th><th>Rozmiar</th><th>Pobrań</th><th>Akcje</th></tr>
    {{range .}}
    <tr>
        <td><img src="/i/{{.Slug}}/thumb" width="50" height="50"></td>
        <td><a href="/i/{{.Slug}}">{{.OriginalName}}</a></td>
        <td>{{.OwnerName}}</td>
        <td>{{printf "%.1f" (divf .FileSize 1024)}} KB</td>
        <td>{{.Downloads}}</td>
        <td>
            <form method="POST" action="/admin/images/{{.ID}}/delete" onsubmit="return confirm('Usunąć zdjęcie?')">
                <button class="btn-delete">Usuń</button>
            </form>
        </td>
    </tr>
    {{end}}
</table>
{{end}}
```

**Step 4: Commit**

```bash
git add internal/storage/db.go internal/handler/admin.go templates/admin/images.html
git commit -m "feat: add images list with delete and download counter in admin panel"
```

---

## Task 11: Routing i integracja

**Files:**
- Modify: `cmd/dajtu/main.go`

**Step 1: Zarejestruj handlery admina**

```go
// Po inicjalizacji innych handlerów:
adminHandler := handler.NewAdminHandler(db, fs)
adminMiddleware := middleware.NewAdminMiddleware(cfg.AdminNicks)

// Routing admina (z middleware)
adminMux := http.NewServeMux()
adminMux.HandleFunc("GET /admin", adminHandler.Dashboard)
adminMux.HandleFunc("GET /admin/users", adminHandler.Users)
adminMux.HandleFunc("GET /admin/galleries", adminHandler.Galleries)
adminMux.HandleFunc("POST /admin/galleries/{id}/delete", adminHandler.DeleteGallery)
adminMux.HandleFunc("GET /admin/images", adminHandler.Images)
adminMux.HandleFunc("POST /admin/images/{id}/delete", adminHandler.DeleteImage)

mux.Handle("/admin", adminMiddleware.Middleware(adminMux))
mux.Handle("/admin/", adminMiddleware.Middleware(adminMux))
```

**Step 2: Test manualny**

```bash
go run ./cmd/dajtu
# Zaloguj się jako admin (KS Amator lub gruby wonsz)
# Otwórz http://localhost:8080/admin
```

**Step 3: Commit**

```bash
git add cmd/dajtu/main.go
git commit -m "feat: wire up admin panel routes with middleware"
```

---

## Task 12: Funkcja pomocnicza dla templates (divf)

**Files:**
- Modify: `internal/handler/admin.go`

**Step 1: Dodaj FuncMap do template**

```go
func NewAdminHandler(db *storage.DB, fs *storage.Filesystem) *AdminHandler {
    funcMap := template.FuncMap{
        "divf": func(a, b int64) float64 {
            return float64(a) / float64(b)
        },
    }
    return &AdminHandler{
        db:   db,
        fs:   fs,
        tmpl: template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/admin/*.html")),
    }
}
```

**Step 2: Commit**

```bash
git add internal/handler/admin.go
git commit -m "feat: add divf template helper for size formatting"
```

---

## Task 13: Dokumentacja w CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Dodaj sekcję Admin Panel**

```markdown
## Admin Panel

Panel dostępny dla użytkowników z listy `ADMIN_NICKS` (domyślnie: "KS Amator", "gruby wonsz").

| Endpoint | Opis |
|----------|------|
| `/admin` | Dashboard ze statystykami |
| `/admin/users` | Lista kont |
| `/admin/galleries` | Lista galerii (z delete) |
| `/admin/images` | Lista zdjęć (z delete i licznikiem pobrań) |

### Konfiguracja

| Zmienna | Default | Opis |
|---------|---------|------|
| `ADMIN_NICKS` | KS Amator,gruby wonsz | Lista nicków adminów (csv) |
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add admin panel section to CLAUDE.md"
```

---

## Podsumowanie zmian

1. **Nowa kolumna `downloads`** w tabeli `images` - licznik pobrań
2. **Aktualizacja `accessed_at`** przy każdym wyświetleniu (istniejąca funkcja `TouchImageBySlug` - tylko nie była wywoływana!)
3. **Zmiana logiki cleanup** - usuwa najdawniej UŻYWANE pliki (`accessed_at`) zamiast najstarszych (`created_at`)
4. **ADMIN_NICKS** env var - lista nicków z dostępem do panelu
5. **Middleware AdminOnly** - sprawdza czy user jest na liście
6. **Handler admina** z endpointami:
   - `/admin` - dashboard ze statystykami
   - `/admin/users` - lista użytkowników
   - `/admin/galleries` - lista galerii + delete
   - `/admin/images` - lista zdjęć + delete
7. **Szablony HTML** minimalistyczne, bez zewnętrznych zależności

## Notatki techniczne

**Licznik pobrań:** Caddy jedynie proxy'uje requesty do Go app - wszystkie ścieżki `/i/*` obsługiwane są przez handler w `main.go`. Licznik i accessed_at aktualizowane asynchronicznie (goroutine) żeby nie spowalniać response.

**SQLite wydajność:** Dla małego/średniego ruchu WAL mode wystarczy. Przy większym ruchu można dodać batch buffer (buforuj w pamięci, flush co 10s).
