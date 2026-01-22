# Admin Panel Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ulepszyć panel admina: darkmode, link w górnej belce dla adminów, linki na dashboardzie, rozszerzone widoki galerii/userów.

**Architecture:**
- Dodać flagę `IsAdmin` do kontekstu index.html i pokazywać link do admina
- Darkmode + DRY: wspólny layout admina (`base.html`) z CSS i navem, a każda podstrona tylko z zawartością
- Dashboard: liczniki jako linki
- Galerie/Userzy: dwa typy linków (publiczny + [ADM])
- Nowe endpointy: `/admin/users/{slug}`, `/admin/galleries/{slug}` z podsumowaniami

**Tech Stack:** Go templates, inline CSS (darkmode)

---

## Task 1: Dodaj link "admin" na górnej belce dla adminów

**Files:**
- Modify: `internal/handler/gallery.go:36-55`
- Modify: `internal/handler/templates/index.html:213-220`
- Modify: `internal/config/config.go` (bez zmian - tylko odczyt)

**Step 1: Zmień gallery.go aby przekazywać IsAdmin do szablonu**

W pliku `internal/handler/gallery.go`, w funkcji `Index` (około linii 36-55), zmień mapowanie userData:

```go
func (h *GalleryHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	var userData map[string]any
	var isAdmin bool
	if user := middleware.GetUser(r); user != nil {
		userData = map[string]any{
			"Slug":        user.Slug,
			"DisplayName": user.DisplayName,
		}
		isAdmin = slices.Contains(h.cfg.AdminNicks, user.DisplayName)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.indexTmpl.ExecuteTemplate(w, "index.html", map[string]any{
		"User":    userData,
		"IsAdmin": isAdmin,
		"Welcome": r.URL.Query().Get("welcome") == "1",
	}); err != nil {
		log.Printf("index template error: %v", err)
	}
}
```

Dodaj import na początku pliku:
```go
import (
	// ... existing imports ...
	"slices"
)
```

**Step 2: Zmień index.html aby pokazywać link do admina**

W pliku `internal/handler/templates/index.html`, w sekcji `.user-info` (linie 213-220), dodaj link do admina:

```html
    {{ if .User }}
    <div class="user-info">
        {{ if .IsAdmin }}<a href="/admin">admin</a>{{ end }}
        <span>Zalogowano jako <a href="/u/{{ .User.Slug }}">{{ .User.DisplayName }}</a></span>
        <form method="post" action="/logout" style="margin:0;">
            <button type="submit">wyloguj</button>
        </form>
    </div>
    {{ end }}
```

**Step 3: Uruchom lokalnie i zweryfikuj**

```bash
pkill -f "./dajtu" 2>/dev/null; sleep 1
set -a && source .env.local && set +a && ./dajtu &
```

Oczekiwany rezultat: Link "admin" widoczny dla zalogowanych adminów obok nicku.

**Step 4: Commit**

```bash
git add internal/handler/gallery.go internal/handler/templates/index.html
git commit -m "feat: show admin link for admin users in top bar"
```

---

## Task 2: Darkmode + DRY dla panelu admina (wspólny layout)

**Files:**
- Create: `internal/handler/templates/admin/base.html`
- Modify: `internal/handler/templates/admin/dashboard.html`
- Modify: `internal/handler/templates/admin/users.html`
- Modify: `internal/handler/templates/admin/galleries.html`
- Modify: `internal/handler/templates/admin/images.html`
- Modify: `internal/handler/admin.go` (parse base + page)

**Step 1: Dodaj wspólny layout admina**

Stwórz plik `internal/handler/templates/admin/base.html`:

```html
{{define "admin_base"}}
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}Admin - dajtu{{end}}</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: system-ui, -apple-system, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; background: #111; color: #fff; min-height: 100vh; }
        nav { margin-bottom: 20px; padding: 10px 15px; background: #1a1a1a; border: 1px solid #333; border-radius: 8px; display: flex; gap: 15px; align-items: center; }
        nav a { color: #4a9eff; text-decoration: none; }
        nav a:hover { text-decoration: underline; }
        nav .spacer { flex: 1; }
        table { width: 100%; border-collapse: collapse; background: #1a1a1a; border: 1px solid #333; border-radius: 8px; overflow: hidden; }
        th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #333; }
        th { background: #222; color: #888; font-weight: 500; }
        tr:last-child td { border-bottom: none; }
        td a { color: #4a9eff; text-decoration: none; }
        td a:hover { text-decoration: underline; }
        h1 { font-weight: 300; margin-bottom: 20px; }
        h1 span { color: #888; font-size: 0.6em; }
        h3 { font-weight: 400; color: #888; margin: 20px 0 10px; }
        .stats { display: flex; flex-wrap: wrap; gap: 15px; margin-top: 20px; }
        .stat { flex: 1; min-width: 200px; padding: 20px; background: #1a1a1a; border: 1px solid #333; border-radius: 8px; text-align: center; }
        .stat a { color: inherit; text-decoration: none; display: block; }
        .stat a:hover { background: #222; border-radius: 8px; }
        .stat-value { font-size: 2.5em; font-weight: bold; color: #4a9eff; }
        .stat-label { color: #888; margin-top: 5px; }
        .adm { color: #888; font-size: 0.85em; margin-left: 8px; }
        .btn-delete { background: #dc3545; color: white; border: none; padding: 6px 12px; cursor: pointer; border-radius: 4px; font-size: 0.9em; }
        .btn-delete:hover { background: #c82333; }
        .thumb { width: 50px; height: 50px; object-fit: cover; border-radius: 4px; }
        .summary { background: #1a1a1a; border: 1px solid #333; border-radius: 8px; padding: 20px; margin-bottom: 20px; }
        .summary h2 { font-weight: 300; margin-bottom: 15px; }
        .summary-stats { display: flex; gap: 30px; flex-wrap: wrap; }
        .summary-stat .value { font-size: 1.5em; color: #4a9eff; font-weight: bold; }
        .summary-stat .label { color: #888; font-size: 0.9em; }
        .public-link { color: #888; font-size: 0.9em; margin-left: 10px; }
    </style>
</head>
<body>
    <nav>
        <a href="/admin">Dashboard</a>
        <a href="/admin/users">Konta</a>
        <a href="/admin/galleries">Galerie</a>
        <a href="/admin/images">Zdjęcia</a>
        <span class="spacer"></span>
        <a href="/">← Powrót</a>
    </nav>
    {{block "content" .}}{{end}}
</body>
</html>
{{end}}
```

**Step 2: Zmień dashboard.html na darkmode + użyj layoutu**

Zamień cały plik `internal/handler/templates/admin/dashboard.html`:

```html
{{define "title"}}Admin - dajtu{{end}}
{{define "content"}}
    <h1>Dashboard</h1>
    <div class="stats">
        <div class="stat">
            <a href="/admin/images">
                <div class="stat-value">{{.TotalImages}}</div>
                <div class="stat-label">Zdjęć</div>
            </a>
        </div>
        <div class="stat">
            <a href="/admin/galleries">
                <div class="stat-value">{{.TotalGalleries}}</div>
                <div class="stat-label">Galerii</div>
            </a>
        </div>
        <div class="stat">
            <a href="/admin/users">
                <div class="stat-value">{{.TotalUsers}}</div>
                <div class="stat-label">Użytkowników</div>
            </a>
        </div>
        <div class="stat">
            <div class="stat-value">{{printf "%.2f" (divf .DiskUsageBytes 1073741824)}} GB</div>
            <div class="stat-label">Zajęte miejsce</div>
        </div>
    </div>
{{end}}
{{template "admin_base" .}}
```

**Step 3: Zmień users.html na darkmode + użyj layoutu**

Zamień cały plik `internal/handler/templates/admin/users.html`:

```html
{{define "title"}}Użytkownicy - Admin dajtu{{end}}
{{define "content"}}
    <h1>Użytkownicy <span>({{len .}})</span></h1>
    <table>
        <tr><th>ID</th><th>Nick</th><th>Slug</th><th>Utworzony</th></tr>
        {{range .}}
        <tr>
            <td>{{.ID}}</td>
            <td>
                <a href="/u/{{.Slug}}">{{.DisplayName}}</a>
                <a href="/admin/users/{{.Slug}}" class="adm">[ADM]</a>
            </td>
            <td>{{.Slug}}</td>
            <td>{{formatDate .CreatedAt}}</td>
        </tr>
        {{else}}
        <tr><td colspan="4" style="color:#666;">Brak użytkowników</td></tr>
        {{end}}
    </table>
{{end}}
{{template "admin_base" .}}
```

**Step 4: Zmień galleries.html na darkmode + użyj layoutu**

Zamień cały plik `internal/handler/templates/admin/galleries.html`:

```html
{{define "title"}}Galerie - Admin dajtu{{end}}
{{define "content"}}
    <h1>Galerie <span>({{len .}})</span></h1>
    <table>
        <tr><th>ID</th><th>Tytuł</th><th>Właściciel</th><th>Zdjęć</th><th>Utworzona</th><th>Akcje</th></tr>
        {{range .}}
        <tr>
            <td>{{.ID}}</td>
            <td>
                <a href="/g/{{.Slug}}" target="_blank">{{if .Title}}{{.Title}}{{else}}(bez tytułu){{end}}</a>
                <a href="/admin/galleries/{{.Slug}}" class="adm">[ADM]</a>
            </td>
            <td>{{if .OwnerSlug}}<a href="/u/{{.OwnerSlug}}">{{.OwnerName}}</a> <a href="/admin/users/{{.OwnerSlug}}" class="adm">[ADM]</a>{{else}}-{{end}}</td>
            <td>{{.ImageCount}}</td>
            <td>{{formatDate .CreatedAt}}</td>
            <td>
                <form method="POST" action="/admin/galleries/{{.ID}}/delete" style="display:inline" onsubmit="return confirm('Usunąć galerię ze wszystkimi zdjęciami?')">
                    <button class="btn-delete">Usuń</button>
                </form>
            </td>
        </tr>
        {{else}}
        <tr><td colspan="6" style="color:#666;">Brak galerii</td></tr>
        {{end}}
    </table>
{{end}}
{{template "admin_base" .}}
```

**Step 5: Zmień images.html na darkmode + użyj layoutu**

Zamień cały plik `internal/handler/templates/admin/images.html`:

```html
{{define "title"}}Zdjęcia - Admin dajtu{{end}}
{{define "content"}}
    <h1>Zdjęcia <span>({{len .}})</span></h1>
    <table>
        <tr><th>Podgląd</th><th>Nazwa</th><th>Właściciel</th><th>Galeria</th><th>Rozmiar</th><th>Pobrań</th><th>Ostatni</th><th>Akcje</th></tr>
        {{range .}}
        <tr>
            <td><a href="/i/{{.Slug}}" target="_blank"><img src="/i/{{.Slug}}/thumb.webp" class="thumb"></a></td>
            <td>{{.OriginalName}}{{if .Edited}} ✏️{{end}}</td>
            <td>{{if .OwnerSlug}}<a href="/u/{{.OwnerSlug}}">{{.OwnerName}}</a> <a href="/admin/users/{{.OwnerSlug}}" class="adm">[ADM]</a>{{else}}-{{end}}</td>
            <td>{{if .GallerySlug}}<a href="/g/{{.GallerySlug}}">{{.GallerySlug}}</a> <a href="/admin/galleries/{{.GallerySlug}}" class="adm">[ADM]</a>{{else}}-{{end}}</td>
            <td>{{printf "%.1f" (divf .FileSize 1024)}} KB</td>
            <td>{{.Downloads}}</td>
            <td>{{formatDate .AccessedAt}}</td>
            <td>
                <form method="POST" action="/admin/images/{{.ID}}/delete" style="display:inline" onsubmit="return confirm('Usunąć zdjęcie?')">
                    <button class="btn-delete">Usuń</button>
                </form>
            </td>
        </tr>
        {{else}}
        <tr><td colspan="8" style="color:#666;">Brak zdjęć</td></tr>
        {{end}}
    </table>
{{end}}
{{template "admin_base" .}}
```

**Step 6: Zmień NewAdminHandler, by parsował base + stronę**

Dodaj pomocniczą funkcję do parsowania templatek z base:

```go
	parseAdmin := func(name, file string) *template.Template {
		return template.Must(template.New(name).Funcs(funcMap).ParseFS(
			templates,
			"templates/admin/base.html",
			file,
		))
	}
```

i użyj jej:

```go
	dashboardTmpl: parseAdmin("dashboard", "templates/admin/dashboard.html"),
	usersTmpl:     parseAdmin("users", "templates/admin/users.html"),
	galleriesTmpl: parseAdmin("galleries", "templates/admin/galleries.html"),
	imagesTmpl:    parseAdmin("images", "templates/admin/images.html"),
```

**Step 7: Commit**

```bash
git add internal/handler/templates/admin/
git commit -m "feat: darkmode for admin panel with linked stats"
```

---

## Task 3: Nowy endpoint - widok admina dla użytkownika

**Files:**
- Modify: `internal/handler/admin.go`
- Create: `internal/handler/templates/admin/user_detail.html`
- Modify: `cmd/dajtu/main.go` (routing)
- Modify: `internal/storage/db.go` (nowe query)

**Pre-fix (DRY + correctness): uzupełnij ImageAdmin o pole Edited**

W `internal/storage/db.go` dodaj pole do structa:

```go
type ImageAdmin struct {
	// ...
	Edited bool
}
```

oraz popraw `ListImagesAdmin`, żeby pobierał `i.edited` i skanował do `img.Edited`.

**Step 1: Dodaj strukturę UserAdmin do db.go**

W pliku `internal/storage/db.go`, po `type ImageAdmin struct` (około linii 627), dodaj:

```go
type UserAdmin struct {
	ID           int64
	Slug         string
	DisplayName  string
	CreatedAt    int64
	UpdatedAt    int64
	GalleryCount int
	ImageCount   int
	TotalSize    int64
	TotalViews   int64
}

func (db *DB) GetUserAdmin(slug string) (*UserAdmin, error) {
	u := &UserAdmin{}
	err := db.conn.QueryRow(`
		SELECT u.id, u.slug, u.display_name, u.created_at, COALESCE(u.updated_at, u.created_at),
		       (SELECT COUNT(*) FROM galleries WHERE user_id = u.id) as gallery_count,
		       (SELECT COUNT(*) FROM images WHERE user_id = u.id) as image_count,
		       (SELECT COALESCE(SUM(file_size), 0) FROM images WHERE user_id = u.id) as total_size,
		       (SELECT COALESCE(SUM(downloads), 0) FROM images WHERE user_id = u.id) as total_views
		FROM users u WHERE u.slug = ?`, slug).Scan(
		&u.ID, &u.Slug, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt,
		&u.GalleryCount, &u.ImageCount, &u.TotalSize, &u.TotalViews)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetUserGalleriesAdmin(userID int64) ([]*GalleryAdmin, error) {
	rows, err := db.conn.Query(`
		SELECT g.id, g.slug, g.title, g.user_id, g.created_at,
		       COUNT(i.id) as image_count,
		       COALESCE(u.display_name, '') as owner_name,
		       COALESCE(u.slug, '') as owner_slug
		FROM galleries g
		LEFT JOIN images i ON i.gallery_id = g.id
		LEFT JOIN users u ON u.id = g.user_id
		WHERE g.user_id = ?
		GROUP BY g.id
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var galleries []*GalleryAdmin
	for rows.Next() {
		g := &GalleryAdmin{}
		if err := rows.Scan(&g.ID, &g.Slug, &g.Title, &g.UserID, &g.CreatedAt, &g.ImageCount, &g.OwnerName, &g.OwnerSlug); err != nil {
			return nil, err
		}
		galleries = append(galleries, g)
	}
	return galleries, rows.Err()
}

func (db *DB) GetUserImagesAdmin(userID int64, limit, offset int) ([]*ImageAdmin, error) {
	rows, err := db.conn.Query(`
		SELECT i.id, i.slug, i.original_name, i.file_size, i.downloads, i.created_at, i.accessed_at, i.edited,
		       COALESCE(u.display_name, '') as owner_name,
		       COALESCE(u.slug, '') as owner_slug,
		       COALESCE(g.slug, '') as gallery_slug
		FROM images i
		LEFT JOIN users u ON u.id = i.user_id
		LEFT JOIN galleries g ON g.id = i.gallery_id
		WHERE i.user_id = ?
		ORDER BY i.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*ImageAdmin
	for rows.Next() {
		img := &ImageAdmin{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.FileSize, &img.Downloads, &img.CreatedAt, &img.AccessedAt, &img.Edited, &img.OwnerName, &img.OwnerSlug, &img.GallerySlug); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}
```

**Step 2: Dodaj handler UserDetail do admin.go**

W pliku `internal/handler/admin.go`, dodaj nowy template i handler:

Zmień struct `AdminHandler` (linie 12-19):

```go
type AdminHandler struct {
	db             *storage.DB
	fs             *storage.Filesystem
	dashboardTmpl  *template.Template
	usersTmpl      *template.Template
	userDetailTmpl *template.Template
	galleriesTmpl  *template.Template
	imagesTmpl     *template.Template
}
```

Zmień `NewAdminHandler` (linie 21-43):

```go
func NewAdminHandler(db *storage.DB, fs *storage.Filesystem) *AdminHandler {
	funcMap := template.FuncMap{
		"divf": func(a, b int64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
		"formatDate": func(ts int64) string {
			if ts == 0 {
				return "-"
			}
			return time.Unix(ts, 0).Format("2006-01-02 15:04")
		},
	}
	parseAdmin := func(name, file string) *template.Template {
		return template.Must(template.New(name).Funcs(funcMap).ParseFS(
			templates,
			"templates/admin/base.html",
			file,
		))
	}
	return &AdminHandler{
		db:             db,
		fs:             fs,
		dashboardTmpl:  parseAdmin("dashboard", "templates/admin/dashboard.html"),
		usersTmpl:      parseAdmin("users", "templates/admin/users.html"),
		userDetailTmpl: parseAdmin("user_detail", "templates/admin/user_detail.html"),
		galleriesTmpl:  parseAdmin("galleries", "templates/admin/galleries.html"),
		imagesTmpl:     parseAdmin("images", "templates/admin/images.html"),
	}
}
```

Dodaj nowy handler na końcu pliku (przed ostatnim `}`):

```go
func (h *AdminHandler) UserDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}

	user, err := h.db.GetUserAdmin(slug)
	if err != nil || user == nil {
		http.NotFound(w, r)
		return
	}

	galleries, _ := h.db.GetUserGalleriesAdmin(user.ID)
	images, _ := h.db.GetUserImagesAdmin(user.ID, 100, 0)

	h.userDetailTmpl.ExecuteTemplate(w, "user_detail.html", map[string]any{
		"User":      user,
		"Galleries": galleries,
		"Images":    images,
	})
}
```

**Step 3: Stwórz template user_detail.html**

Stwórz plik `internal/handler/templates/admin/user_detail.html`:

```html
{{define "title"}}{{.User.DisplayName}} - Admin dajtu{{end}}
{{define "content"}}
    <h1>{{.User.DisplayName}} <a href="/u/{{.User.Slug}}" class="public-link">→ profil publiczny</a></h1>

    <div class="summary">
        <h2>Podsumowanie</h2>
        <div class="summary-stats">
            <div class="summary-stat">
                <div class="value">{{.User.GalleryCount}}</div>
                <div class="label">galerii</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{.User.ImageCount}}</div>
                <div class="label">zdjęć</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{printf "%.2f" (divf .User.TotalSize 1048576)}} MB</div>
                <div class="label">zajęte miejsce</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{.User.TotalViews}}</div>
                <div class="label">łącznie pobrań</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{formatDate .User.CreatedAt}}</div>
                <div class="label">utworzony</div>
            </div>
        </div>
    </div>

    <h3>Galerie ({{len .Galleries}})</h3>
    <table>
        <tr><th>ID</th><th>Tytuł</th><th>Zdjęć</th><th>Utworzona</th><th>Akcje</th></tr>
        {{range .Galleries}}
        <tr>
            <td>{{.ID}}</td>
            <td>
                <a href="/g/{{.Slug}}" target="_blank">{{if .Title}}{{.Title}}{{else}}(bez tytułu){{end}}</a>
                <a href="/admin/galleries/{{.Slug}}" class="adm">[ADM]</a>
            </td>
            <td>{{.ImageCount}}</td>
            <td>{{formatDate .CreatedAt}}</td>
            <td>
                <form method="POST" action="/admin/galleries/{{.ID}}/delete" style="display:inline" onsubmit="return confirm('Usunąć galerię ze wszystkimi zdjęciami?')">
                    <button class="btn-delete">Usuń</button>
                </form>
            </td>
        </tr>
        {{else}}
        <tr><td colspan="5" style="color:#666;">Brak galerii</td></tr>
        {{end}}
    </table>

    <h3>Zdjęcia ({{len .Images}})</h3>
    <table>
        <tr><th>Podgląd</th><th>Nazwa</th><th>Galeria</th><th>Rozmiar</th><th>Pobrań</th><th>Akcje</th></tr>
        {{range .Images}}
        <tr>
            <td><a href="/i/{{.Slug}}" target="_blank"><img src="/i/{{.Slug}}/thumb.webp" class="thumb"></a></td>
            <td>{{.OriginalName}}</td>
            <td>{{if .GallerySlug}}<a href="/g/{{.GallerySlug}}">{{.GallerySlug}}</a> <a href="/admin/galleries/{{.GallerySlug}}" class="adm">[ADM]</a>{{else}}-{{end}}</td>
            <td>{{printf "%.1f" (divf .FileSize 1024)}} KB</td>
            <td>{{.Downloads}}</td>
            <td>
                <form method="POST" action="/admin/images/{{.ID}}/delete" style="display:inline" onsubmit="return confirm('Usunąć zdjęcie?')">
                    <button class="btn-delete">Usuń</button>
                </form>
            </td>
        </tr>
        {{else}}
        <tr><td colspan="6" style="color:#666;">Brak zdjęć</td></tr>
        {{end}}
    </table>
{{end}}
{{template "admin_base" .}}
```

**Step 4: Dodaj routing w main.go**

W pliku `cmd/dajtu/main.go`, w sekcji adminMux (około linii 111-118), dodaj nowy route:

```go
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /admin", adminHandler.Dashboard)
	adminMux.HandleFunc("GET /admin/users", adminHandler.Users)
	adminMux.HandleFunc("GET /admin/users/{slug}", adminHandler.UserDetail)
	adminMux.HandleFunc("GET /admin/galleries", adminHandler.Galleries)
	adminMux.HandleFunc("POST /admin/galleries/{id}/delete", adminHandler.DeleteGallery)
	adminMux.HandleFunc("GET /admin/images", adminHandler.Images)
	adminMux.HandleFunc("POST /admin/images/{id}/delete", adminHandler.DeleteImage)
```

**Step 5: Commit**

```bash
git add internal/storage/db.go internal/handler/admin.go internal/handler/templates/admin/user_detail.html cmd/dajtu/main.go
git commit -m "feat: add admin user detail view with stats"
```

---

## Task 4: Nowy endpoint - widok admina dla galerii

**Files:**
- Modify: `internal/handler/admin.go`
- Create: `internal/handler/templates/admin/gallery_detail.html`
- Modify: `cmd/dajtu/main.go`
- Modify: `internal/storage/db.go`

**Step 1: Dodaj strukturę GalleryDetailAdmin do db.go**

W pliku `internal/storage/db.go`, po `type UserAdmin struct`, dodaj:

```go
type GalleryDetailAdmin struct {
	ID          int64
	Slug        string
	Title       string
	Description string
	UserID      *int64
	OwnerName   string
	OwnerSlug   string
	CreatedAt   int64
	UpdatedAt   int64
	ImageCount  int
	TotalSize   int64
	TotalViews  int64
}

func (db *DB) GetGalleryAdmin(slug string) (*GalleryDetailAdmin, error) {
	g := &GalleryDetailAdmin{}
	var ownerID sql.NullInt64
	err := db.conn.QueryRow(`
		SELECT g.id, g.slug, g.title, g.description, g.user_id,
		       COALESCE(u.display_name, '') as owner_name,
		       COALESCE(u.slug, '') as owner_slug,
		       g.created_at, COALESCE(g.updated_at, g.created_at),
		       (SELECT COUNT(*) FROM images WHERE gallery_id = g.id) as image_count,
		       (SELECT COALESCE(SUM(file_size), 0) FROM images WHERE gallery_id = g.id) as total_size,
		       (SELECT COALESCE(SUM(downloads), 0) FROM images WHERE gallery_id = g.id) as total_views
		FROM galleries g
		LEFT JOIN users u ON u.id = g.user_id
		WHERE g.slug = ?`, slug).Scan(
		&g.ID, &g.Slug, &g.Title, &g.Description, &ownerID,
		&g.OwnerName, &g.OwnerSlug, &g.CreatedAt, &g.UpdatedAt,
		&g.ImageCount, &g.TotalSize, &g.TotalViews)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if ownerID.Valid {
		g.UserID = &ownerID.Int64
	}
	return g, err
}

func (db *DB) GetGalleryImagesAdmin(galleryID int64) ([]*ImageAdmin, error) {
	rows, err := db.conn.Query(`
		SELECT i.id, i.slug, i.original_name, i.file_size, i.downloads, i.created_at, i.accessed_at, i.edited,
		       COALESCE(u.display_name, '') as owner_name,
		       COALESCE(u.slug, '') as owner_slug,
		       COALESCE(g.slug, '') as gallery_slug
		FROM images i
		LEFT JOIN users u ON u.id = i.user_id
		LEFT JOIN galleries g ON g.id = i.gallery_id
		WHERE i.gallery_id = ?
		ORDER BY i.created_at DESC
	`, galleryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*ImageAdmin
	for rows.Next() {
		img := &ImageAdmin{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.FileSize, &img.Downloads, &img.CreatedAt, &img.AccessedAt, &img.Edited, &img.OwnerName, &img.OwnerSlug, &img.GallerySlug); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}
```

**Step 2: Dodaj handler GalleryDetail do admin.go**

Zmień struct `AdminHandler`:

```go
type AdminHandler struct {
	db                *storage.DB
	fs                *storage.Filesystem
	dashboardTmpl     *template.Template
	usersTmpl         *template.Template
	userDetailTmpl    *template.Template
	galleriesTmpl     *template.Template
	galleryDetailTmpl *template.Template
	imagesTmpl        *template.Template
}
```

Dodaj w `NewAdminHandler`:

```go
		galleryDetailTmpl: parseAdmin("gallery_detail", "templates/admin/gallery_detail.html"),
```

Dodaj handler:

```go
func (h *AdminHandler) GalleryDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}

	gallery, err := h.db.GetGalleryAdmin(slug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	images, _ := h.db.GetGalleryImagesAdmin(gallery.ID)

	h.galleryDetailTmpl.ExecuteTemplate(w, "gallery_detail.html", map[string]any{
		"Gallery": gallery,
		"Images":  images,
	})
}
```

**Step 3: Stwórz template gallery_detail.html**

Stwórz plik `internal/handler/templates/admin/gallery_detail.html`:

```html
{{define "title"}}{{if .Gallery.Title}}{{.Gallery.Title}}{{else}}Galeria {{.Gallery.Slug}}{{end}} - Admin dajtu{{end}}
{{define "content"}}
    <h1>{{if .Gallery.Title}}{{.Gallery.Title}}{{else}}Galeria {{.Gallery.Slug}}{{end}} <a href="/g/{{.Gallery.Slug}}" class="public-link">→ widok publiczny</a></h1>

    <div class="summary">
        <h2>Podsumowanie</h2>
        <div class="summary-stats">
            <div class="summary-stat">
                <div class="value">{{.Gallery.ImageCount}}</div>
                <div class="label">zdjęć</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{printf "%.2f" (divf .Gallery.TotalSize 1048576)}} MB</div>
                <div class="label">łączny rozmiar</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{.Gallery.TotalViews}}</div>
                <div class="label">łącznie pobrań</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{if .Gallery.OwnerSlug}}<a href="/u/{{.Gallery.OwnerSlug}}">{{.Gallery.OwnerName}}</a> <a href="/admin/users/{{.Gallery.OwnerSlug}}" class="adm">[ADM]</a>{{else}}-{{end}}</div>
                <div class="label">właściciel</div>
            </div>
            <div class="summary-stat">
                <div class="value">{{formatDate .Gallery.CreatedAt}}</div>
                <div class="label">utworzona</div>
            </div>
        </div>
    </div>

    <h3>Zdjęcia ({{len .Images}})</h3>
    <table>
        <tr><th>Podgląd</th><th>Nazwa</th><th>Rozmiar</th><th>Pobrań</th><th>Ostatni</th><th>Akcje</th></tr>
        {{range .Images}}
        <tr>
            <td><a href="/i/{{.Slug}}" target="_blank"><img src="/i/{{.Slug}}/thumb.webp" class="thumb"></a></td>
            <td>{{.OriginalName}}</td>
            <td>{{printf "%.1f" (divf .FileSize 1024)}} KB</td>
            <td>{{.Downloads}}</td>
            <td>{{formatDate .AccessedAt}}</td>
            <td>
                <form method="POST" action="/admin/images/{{.ID}}/delete" style="display:inline" onsubmit="return confirm('Usunąć zdjęcie?')">
                    <button class="btn-delete">Usuń</button>
                </form>
            </td>
        </tr>
        {{else}}
        <tr><td colspan="6" style="color:#666;">Brak zdjęć</td></tr>
        {{end}}
    </table>

    <div style="margin-top: 20px;">
        <form method="POST" action="/admin/galleries/{{.Gallery.ID}}/delete" style="display:inline" onsubmit="return confirm('Usunąć galerię ze wszystkimi zdjęciami?')">
            <button class="btn-delete">Usuń całą galerię</button>
        </form>
    </div>
{{end}}
{{template "admin_base" .}}
```

**Step 4: Dodaj routing w main.go**

```go
	adminMux.HandleFunc("GET /admin/galleries/{slug}", adminHandler.GalleryDetail)
```

**Step 5: Commit**

```bash
git add internal/storage/db.go internal/handler/admin.go internal/handler/templates/admin/gallery_detail.html cmd/dajtu/main.go
git commit -m "feat: add admin gallery detail view with stats"
```

---

## Task 5: Test lokalny i deploy

**Step 1: Uruchom lokalnie**

```bash
pkill -f "./dajtu" 2>/dev/null; sleep 1
go build -o dajtu ./cmd/dajtu && set -a && source .env.local && set +a && ./dajtu &
```

**Step 2: Przetestuj w przeglądarce**

- http://localhost:8085/ - sprawdź link "admin" dla zalogowanego admina
- http://localhost:8085/admin - dashboard z darkmode
- http://localhost:8085/admin/users - lista użytkowników z linkami [ADM]
- http://localhost:8085/admin/galleries - lista galerii z linkami [ADM]
- http://localhost:8085/admin/images - lista zdjęć z linkami [ADM]
- http://localhost:8085/admin/users/{slug} - szczegóły użytkownika
- http://localhost:8085/admin/galleries/{slug} - szczegóły galerii

**Step 3: Deploy na staging**

```bash
git push
ssh staging "cd /var/www/dajtu && git pull && docker compose up --build -d"
```

**Step 4: Zweryfikuj na staging**

```bash
ssh staging "docker logs dajtu_app --tail 20"
```
