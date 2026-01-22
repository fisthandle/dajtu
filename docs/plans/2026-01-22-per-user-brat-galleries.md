# Per-User Braterstwo Galleries Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Każdy użytkownik Braterstwa ma osobną galerię dla każdego tematu - nie współdzieloną.

**Architecture:** Zmiana `external_id` z `brat-{entryID}` na `brat-{entryID}-{userSlug}`. Dodanie metody `GetOrCreateBratGallery` która szuka galerii po external_id + user_id. Migracja nie jest potrzebna - istniejące galerie pozostają, nowe będą per-user.

**Tech Stack:** Go, SQLite

---

## Analiza obecnego stanu

**Problem:** `GetGalleryByExternalID("brat-123")` zwraca pierwszą galerię dla tematu 123, niezależnie od użytkownika. Wszyscy użytkownicy wrzucają do tej samej galerii.

**Rozwiązanie:** Zmienić external_id na format `brat-{entryID}-{userSlug}` i szukać galerii po tym unikatowym kluczu.

---

### Task 1: Dodaj metodę GetOrCreateBratGallery

**Files:**
- Modify: `internal/storage/db.go` (po linii 248)

**Step 1: Dodaj nową metodę do db.go**

```go
func (db *DB) GetOrCreateBratGallery(userID int64, userSlug, entryID, title string) (*Gallery, error) {
	externalID := fmt.Sprintf("brat-%s-%s", entryID, userSlug)

	g := &Gallery{}
	err := db.conn.QueryRow(`
		SELECT id, slug, edit_token, title, description, user_id, external_id, created_at, updated_at
		FROM galleries WHERE external_id = ? AND user_id = ?`, externalID, userID).Scan(
		&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt)

	if err == nil {
		return g, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	slug := db.GenerateUniqueSlug("galleries", 4)
	editToken := generateSecureToken()
	now := time.Now().Unix()

	res, err := db.conn.Exec(`
		INSERT INTO galleries (slug, edit_token, title, description, user_id, external_id, created_at, updated_at)
		VALUES (?, ?, ?, '', ?, ?, ?, ?)`,
		slug, editToken, title, userID, externalID, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return &Gallery{
		ID:         id,
		Slug:       slug,
		EditToken:  editToken,
		Title:      title,
		UserID:     &userID,
		ExternalID: &externalID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func generateSecureToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
```

**Step 2: Dodaj import `crypto/rand` i `encoding/base64` do db.go**

Sprawdź istniejące importy i dodaj brakujące.

**Step 3: Zbuduj i sprawdź błędy**

```bash
go build ./...
```

Expected: Build passes

**Step 4: Commit**

```bash
git add internal/storage/db.go
git commit -m "feat: add GetOrCreateBratGallery for per-user galleries"
```

---

### Task 2: Zaktualizuj BratUploadHandler

**Files:**
- Modify: `internal/handler/brat_upload.go:118-155`

**Step 1: Zamień logikę tworzenia galerii**

Znajdź ten blok (linie 118-155):
```go
dbUser, err := h.db.GetOrCreateBratUser(user.Pseudonim)
if err != nil {
    log.Printf("get/create user error: %v", err)
    jsonError(w, "user creation failed", http.StatusInternalServerError)
    return
}

externalID := fmt.Sprintf("brat-%s", entryID)
gallery, err := h.db.GetGalleryByExternalID(externalID)
// ... reszta tworzenia galerii
```

Zamień na:
```go
dbUser, err := h.db.GetOrCreateBratUser(user.Pseudonim)
if err != nil {
    log.Printf("get/create user error: %v", err)
    jsonError(w, "user creation failed", http.StatusInternalServerError)
    return
}

gallery, err := h.db.GetOrCreateBratGallery(dbUser.ID, dbUser.Slug, entryID, title)
if err != nil {
    log.Printf("get/create gallery error: %v", err)
    jsonError(w, "gallery creation failed", http.StatusInternalServerError)
    return
}
```

**Step 2: Usuń nieużywany import `fmt` jeśli był tylko do externalID**

Sprawdź czy `fmt` jest używany gdzie indziej w pliku.

**Step 3: Zbuduj i sprawdź błędy**

```bash
go build ./...
```

Expected: Build passes

**Step 4: Uruchom testy**

```bash
go test ./...
```

Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/handler/brat_upload.go
git commit -m "refactor: use per-user galleries for Braterstwo uploads"
```

---

### Task 3: Deploy i weryfikacja

**Step 1: Push i deploy**

```bash
git push
ssh staging "cd /var/www/dajtu && git pull && docker compose up --build -d"
```

**Step 2: Sprawdź logi**

```bash
ssh staging "docker logs dajtu_app --tail 50"
```

Expected: No errors on startup

**Step 3: Commit podsumowujący (opcjonalnie)**

Jeśli wszystko działa, można połączyć commity lub zostawić jako są.

---

## Podsumowanie zmian

| Komponent | Zmiana |
|-----------|--------|
| `db.go` | Nowa metoda `GetOrCreateBratGallery(userID, userSlug, entryID, title)` |
| `brat_upload.go` | Użycie nowej metody zamiast `GetGalleryByExternalID` |
| Format external_id | `brat-{entryID}` → `brat-{entryID}-{userSlug}` |

## Backward Compatibility

- Istniejące galerie z formatem `brat-{entryID}` pozostają nietknięte
- Nowe uploady tworzą galerie z formatem `brat-{entryID}-{userSlug}`
- Użytkownicy którzy już wrzucali do wspólnej galerii, dostaną nową osobną galerię przy następnym uploadzie
