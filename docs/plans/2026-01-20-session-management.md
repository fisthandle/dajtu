# Session Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Dodać sesje użytkowników po SSO callback - cookie z tokenem, middleware do autoryzacji, powiązanie galerii z userem, profil z listą galerii.

**Architecture:** HttpOnly cookie `session` z 32-bajtowym tokenem → tabela `sessions(token, user_id, expires_at)` w SQLite → middleware wyciąga usera z cookie → handlery używają usera do przypisania galerii.

**Tech Stack:** Go stdlib (net/http, crypto/rand), SQLite, HttpOnly cookies

---

### Task 1: Session Storage Layer

**Files:**
- Modify: `internal/storage/db.go:72-125` (migrate + nowe metody)

**Step 1: Dodaj migrację tabeli sessions**

W funkcji `migrate()` po linii 119 (przed ostatnim `}`) dodaj:

```go
	CREATE TABLE IF NOT EXISTS sessions (
		token CHAR(64) PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
```

**Step 2: Dodaj struct Session i metody**

Po `type Gallery struct` (linia 51) dodaj:

```go
type Session struct {
	Token     string
	UserID    int64
	ExpiresAt int64
	CreatedAt int64
}
```

Na końcu pliku dodaj metody:

```go
func (db *DB) CreateSession(userID int64, durationDays int) (*Session, error) {
	token := GenerateSlug(64)
	now := time.Now().Unix()
	expiresAt := now + int64(durationDays*24*60*60)

	_, err := db.conn.Exec(
		`INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		token, userID, expiresAt, now,
	)
	if err != nil {
		return nil, err
	}

	return &Session{Token: token, UserID: userID, ExpiresAt: expiresAt, CreatedAt: now}, nil
}

func (db *DB) GetSession(token string) (*Session, error) {
	var s Session
	err := db.conn.QueryRow(
		`SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ? AND expires_at > ?`,
		token, time.Now().Unix(),
	).Scan(&s.Token, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) DeleteSession(token string) error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (db *DB) DeleteUserSessions(userID int64) error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (db *DB) CleanExpiredSessions() (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
```

**Step 3: Zbuduj i sprawdź czy kompiluje**

Run: `go build ./...`
Expected: brak błędów

**Step 4: Commit**

```bash
git add internal/storage/db.go
git commit -m "feat(storage): add sessions table and CRUD methods"
```

---

### Task 2: Auth Handler - Cookie po SSO

**Files:**
- Modify: `internal/handler/auth.go:38-80`

**Step 1: Zmodyfikuj HandleBratSSO żeby ustawiał cookie**

Zamień funkcję `HandleBratSSO` (linie 38-80) na:

```go
func (h *AuthHandler) HandleBratSSO(w http.ResponseWriter, r *http.Request) {
	if h.decoder == nil {
		http.Error(w, "SSO not configured", http.StatusServiceUnavailable)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/auth/brat/")
	data := strings.TrimSuffix(path, "/")
	if data == "" {
		data = r.URL.Query().Get("data")
	}
	if data == "" {
		http.Error(w, "missing data parameter", http.StatusBadRequest)
		return
	}

	user, err := h.decoder.Decode(data)
	if err != nil {
		log.Printf("SSO decode error: %v", err)
		http.Error(w, "invalid SSO payload", http.StatusUnauthorized)
		return
	}

	dbUser, err := h.db.GetOrCreateBratUser(user.Pseudonim, user.Punktacja)
	if err != nil {
		log.Printf("SSO user error: %v", err)
		http.Error(w, "user creation failed", http.StatusInternalServerError)
		return
	}

	// Create session (30 days)
	session, err := h.db.CreateSession(dbUser.ID, 30)
	if err != nil {
		log.Printf("Session creation error: %v", err)
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}

	// Set HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.Token,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   strings.HasPrefix(h.cfg.BaseURL, "https"),
		SameSite: http.SameSiteLaxMode,
	})

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"user_id":   dbUser.ID,
			"slug":      dbUser.Slug,
			"name":      dbUser.DisplayName,
			"punktacja": user.Punktacja,
		})
		return
	}

	http.Redirect(w, r, "/u/"+dbUser.Slug, http.StatusFound)
}
```

**Step 2: Dodaj endpoint logout**

Na końcu pliku dodaj:

```go
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		h.db.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}
```

**Step 3: Zbuduj**

Run: `go build ./...`
Expected: brak błędów

**Step 4: Commit**

```bash
git add internal/handler/auth.go
git commit -m "feat(auth): set session cookie after SSO callback, add logout"
```

---

### Task 3: Session Middleware

**Files:**
- Create: `internal/middleware/session.go`

**Step 1: Utwórz middleware**

```go
package middleware

import (
	"context"
	"net/http"

	"dajtu/internal/storage"
)

type contextKey string

const UserContextKey contextKey = "user"

type SessionMiddleware struct {
	db *storage.DB
}

func NewSessionMiddleware(db *storage.DB) *SessionMiddleware {
	return &SessionMiddleware{db: db}
}

// GetUser extracts user from request context (set by middleware)
func GetUser(r *http.Request) *storage.User {
	user, _ := r.Context().Value(UserContextKey).(*storage.User)
	return user
}

// Middleware adds user to context if valid session exists
func (m *SessionMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err == nil && cookie.Value != "" {
			session, err := m.db.GetSession(cookie.Value)
			if err == nil {
				user, err := m.db.GetUserByID(session.UserID)
				if err == nil && user != nil {
					ctx := context.WithValue(r.Context(), UserContextKey, user)
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
```

**Step 2: Dodaj GetUserByID do db.go**

W `internal/storage/db.go` po metodzie `GetUserBySlug` dodaj:

```go
func (db *DB) GetUserByID(id int64) (*User, error) {
	var u User
	err := db.conn.QueryRow(
		`SELECT id, slug, display_name, brat_pseudo, brat_punktacja, created_at, updated_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Slug, &u.DisplayName, &u.BratPseudo, &u.BratPunktacja, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
```

**Step 3: Zbuduj**

Run: `go build ./...`
Expected: brak błędów

**Step 4: Commit**

```bash
git add internal/middleware/session.go internal/storage/db.go
git commit -m "feat(middleware): add session middleware with user context"
```

---

### Task 4: Zarejestruj middleware i logout w main.go

**Files:**
- Modify: `cmd/dajtu/main.go`

**Step 1: Dodaj session middleware i logout route**

Po linii 41 (uploadLimiter) dodaj:

```go
	sessionMiddleware := middleware.NewSessionMiddleware(db)
```

Po linii 79 (auth/brat/) dodaj:

```go
	mux.HandleFunc("/logout", authHandler.Logout)
```

Zmień wrapper mux na session middleware. Zamień linię 120-122:

```go
	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
```

na:

```go
	log.Printf("Starting server on :%s", cfg.Port)
	handler := sessionMiddleware.Middleware(mux)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
	}
```

**Step 2: Zbuduj**

Run: `go build ./...`
Expected: brak błędów

**Step 3: Commit**

```bash
git add cmd/dajtu/main.go
git commit -m "feat(main): wire session middleware and logout route"
```

---

### Task 5: Przypisywanie user_id do galerii

**Files:**
- Modify: `internal/handler/gallery.go:52-90`

**Step 1: Zmodyfikuj Gallery.Create żeby przypisywał user_id**

W funkcji `Create` (linia 52) zmień tworzenie gallery (linie 76-83) na:

```go
	var userID *int64
	if user := middleware.GetUser(r); user != nil {
		userID = &user.ID
	}

	gallery := &storage.Gallery{
		Slug:        gallerySlug,
		EditToken:   editToken,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		UserID:      userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
```

Dodaj import na górze pliku:

```go
	"dajtu/internal/middleware"
```

**Step 2: Zbuduj**

Run: `go build ./...`
Expected: brak błędów

**Step 3: Commit**

```bash
git add internal/handler/gallery.go
git commit -m "feat(gallery): assign user_id to galleries when logged in"
```

---

### Task 6: Profil z listą galerii

**Files:**
- Modify: `internal/storage/db.go` (dodaj GetUserGalleries)
- Modify: `internal/handler/user.go`
- Modify: `internal/handler/templates/user.html`

**Step 1: Dodaj GetUserGalleries do db.go**

```go
func (db *DB) GetUserGalleries(userID int64) ([]*Gallery, error) {
	rows, err := db.conn.Query(
		`SELECT id, slug, edit_token, title, description, user_id, created_at, updated_at
		 FROM galleries WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var galleries []*Gallery
	for rows.Next() {
		g := &Gallery{}
		err := rows.Scan(&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, err
		}
		galleries = append(galleries, g)
	}
	return galleries, nil
}
```

**Step 2: Zaktualizuj user handler**

Zamień całą funkcję `View` w `internal/handler/user.go`:

```go
func (h *UserHandler) View(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/u/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	if len(parts) > 1 && parts[1] != "" {
		http.NotFound(w, r)
		return
	}
	slug := parts[0]

	user, err := h.db.GetUserBySlug(slug)
	if err != nil || user == nil {
		http.NotFound(w, r)
		return
	}

	galleries, _ := h.db.GetUserGalleries(user.ID)

	type GalleryData struct {
		Slug      string
		Title     string
		URL       string
		CreatedAt string
	}

	var galleryList []GalleryData
	for _, g := range galleries {
		title := g.Title
		if title == "" {
			title = "Galeria " + g.Slug
		}
		galleryList = append(galleryList, GalleryData{
			Slug:      g.Slug,
			Title:     title,
			URL:       h.cfg.BaseURL + "/g/" + g.Slug,
			CreatedAt: time.Unix(g.CreatedAt, 0).Format("2006-01-02 15:04"),
		})
	}

	data := map[string]any{
		"Slug":        user.Slug,
		"DisplayName": user.DisplayName,
		"Punktacja":   user.BratPunktacja,
		"Galleries":   galleryList,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.userTmpl.Execute(w, data)
}
```

Dodaj import `"time"` na górze pliku.

**Step 3: Zaktualizuj template**

Zamień `internal/handler/templates/user.html`:

```html
<!DOCTYPE html>
<html lang="pl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>dajtu.com - {{ .DisplayName }}</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: system-ui, -apple-system, sans-serif;
            background: #111;
            color: #fff;
            min-height: 100vh;
            padding: 40px 20px;
        }
        .container { max-width: 800px; margin: 0 auto; }
        h1 { font-size: 2.2rem; font-weight: 300; margin-bottom: 8px; }
        .subtitle { color: #777; margin-bottom: 30px; }
        .card {
            background: #1a1a1a;
            border: 1px solid #333;
            border-radius: 12px;
            padding: 24px;
            margin-bottom: 20px;
        }
        .row {
            display: flex;
            justify-content: space-between;
            padding: 10px 0;
            border-bottom: 1px solid #222;
        }
        .row:last-child { border-bottom: none; }
        .label { color: #777; font-size: 0.9rem; }
        .value { font-weight: 500; }
        h2 { font-size: 1.4rem; font-weight: 400; margin: 30px 0 15px; }
        .gallery-list { list-style: none; }
        .gallery-item {
            background: #1a1a1a;
            border: 1px solid #333;
            border-radius: 8px;
            padding: 16px;
            margin-bottom: 10px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .gallery-item a {
            color: #4a9eff;
            text-decoration: none;
            font-weight: 500;
        }
        .gallery-item a:hover { text-decoration: underline; }
        .gallery-date { color: #666; font-size: 0.85rem; }
        .empty { color: #666; font-style: italic; }
    </style>
</head>
<body>
    <div class="container">
        <h1>{{ .DisplayName }}</h1>
        <p class="subtitle">Profil użytkownika</p>

        <div class="card">
            <div class="row">
                <span class="label">Slug</span>
                <span class="value">{{ .Slug }}</span>
            </div>
            <div class="row">
                <span class="label">Punktacja</span>
                <span class="value">{{ .Punktacja }}</span>
            </div>
        </div>

        <h2>Galerie</h2>
        {{ if .Galleries }}
        <ul class="gallery-list">
            {{ range .Galleries }}
            <li class="gallery-item">
                <a href="{{ .URL }}">{{ .Title }}</a>
                <span class="gallery-date">{{ .CreatedAt }}</span>
            </li>
            {{ end }}
        </ul>
        {{ else }}
        <p class="empty">Brak galerii</p>
        {{ end }}
    </div>
</body>
</html>
```

**Step 4: Zbuduj**

Run: `go build ./...`
Expected: brak błędów

**Step 5: Commit**

```bash
git add internal/storage/db.go internal/handler/user.go internal/handler/templates/user.html
git commit -m "feat(user): show user galleries on profile page"
```

---

### Task 7: Link do logowania na stronie głównej

**Files:**
- Modify: `internal/handler/gallery.go:35-42`
- Modify: `internal/handler/templates/index.html`

**Step 1: Przekaż user do template w Index**

Zamień funkcję `Index` w gallery.go:

```go
func (h *GalleryHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	var userData map[string]any
	if user := middleware.GetUser(r); user != nil {
		userData = map[string]any{
			"Slug":        user.Slug,
			"DisplayName": user.DisplayName,
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.indexTmpl.Execute(w, map[string]any{
		"User": userData,
	})
}
```

**Step 2: Dodaj header z logowaniem do index.html**

Po `<body>` (linia 139) dodaj:

```html
    <header style="position:fixed;top:0;right:0;padding:15px 20px;">
        {{ if .User }}
        <a href="/u/{{ .User.Slug }}" style="color:#4a9eff;text-decoration:none;margin-right:15px;">{{ .User.DisplayName }}</a>
        <a href="/logout" style="color:#888;text-decoration:none;">Wyloguj</a>
        {{ else }}
        <a href="https://braterstwo.net/sso/dajtu" style="color:#4a9eff;text-decoration:none;">Zaloguj przez Braterstwo</a>
        {{ end }}
    </header>
```

**Step 3: Zbuduj**

Run: `go build ./...`
Expected: brak błędów

**Step 4: Commit**

```bash
git add internal/handler/gallery.go internal/handler/templates/index.html
git commit -m "feat(index): add login/logout header with user info"
```

---

### Task 8: Deploy i test

**Step 1: Push**

```bash
git push origin master
```

**Step 2: Deploy na serwer**

```bash
ssh staging "cd /var/www/dajtu && git pull && docker compose up --build -d"
```

**Step 3: Sprawdź logi**

```bash
ssh staging "docker logs dajtu_app --tail 20"
```

**Step 4: Test flow**

1. Otwórz https://dajtu.com - sprawdź czy jest link "Zaloguj przez Braterstwo"
2. Zaloguj się przez Braterstwo
3. Sprawdź czy po redirect jesteś na /u/{slug} i widzisz profil
4. Sprawdź czy na / widać twój nick w headerze
5. Utwórz galerię
6. Wróć na profil - sprawdź czy galeria jest widoczna
7. Kliknij "Wyloguj" - sprawdź czy cookie zniknęło
