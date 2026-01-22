# View Page Improvements - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ulepszyć strony podglądu obrazka i galerii - dodać pełne informacje (linki, edit token), możliwość usunięcia, logo dajtu na górze.

**Architecture:**
- Obrazek pojedynczy: dodać edit_token do response uploadu, wyświetlić na stronie podglądu, dodać przycisk delete
- Galeria: dodać sekcję z linkami i edit token na widoku galerii
- Oba: logo dajtu u góry jako link do strony głównej
- Delete obrazka: nowy endpoint DELETE /i/{slug} z autoryzacją przez edit_token

**Tech Stack:** Go handlers, HTML templates, vanilla JS

---

## Task 1: Dodaj edit_token do response uploadu obrazka

**Kontekst:** Obecnie `/upload` zwraca tylko `{slug, url, sizes}`. Potrzebujemy też `edit_token` żeby użytkownik mógł później edytować/usuwać.

**Files:**
- Modify: `internal/storage/db.go` - dodać EditToken do Image struct
- Modify: `internal/handler/upload.go:186-193` - dodać edit_token do response

**Step 1: Dodaj EditToken do Image struct**

W `internal/storage/db.go` znajdź struct Image i dodaj pole:
```go
type Image struct {
    // ... existing fields ...
    EditToken    string `json:"edit_token,omitempty"`
}
```

**Step 2: Wygeneruj edit_token przy insercie**

W `internal/handler/upload.go` przed `h.db.InsertImage(img)`:
```go
img.EditToken = generateToken(32)
```

Gdzie `generateToken` generuje random hex string (może już istnieć w kodzie - sprawdź `internal/handler/gallery.go`).

**Step 3: Zwróć edit_token w response**

W `internal/handler/upload.go` zmodyfikuj UploadResponse:
```go
type UploadResponse struct {
    Slug      string            `json:"slug"`
    URL       string            `json:"url,omitempty"`
    Sizes     map[string]string `json:"sizes,omitempty"`
    EditToken string            `json:"edit_token,omitempty"`
}
```

I w ServeHTTP dodaj do resp:
```go
resp := UploadResponse{
    Slug:      slug,
    URL:       sizes["original"],
    Sizes:     sizes,
    EditToken: img.EditToken,
}
```

**Step 4: Update migracji DB**

Sprawdź czy tabela images ma kolumnę edit_token. Jeśli nie, dodaj ją w `internal/storage/db.go` w funkcji tworzącej tabelę.

**Step 5: Build i test**

```bash
go build -o dajtu ./cmd/dajtu
```

---

## Task 2: Endpoint DELETE /i/{slug} dla pojedynczego obrazka

**Kontekst:** Obecnie nie ma endpointu do usuwania pojedynczego obrazka (jest tylko w galerii i adminie).

**Files:**
- Modify: `cmd/dajtu/main.go` - dodać routing dla DELETE /i/{slug}
- Modify: `internal/handler/upload.go` - dodać DeleteImage handler

**Step 1: Dodaj handler DeleteImage**

W `internal/handler/upload.go` dodaj nową metodę:
```go
func (h *UploadHandler) DeleteImage(w http.ResponseWriter, r *http.Request, slug string) {
    if r.Method != http.MethodDelete {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Get edit token from header or form
    editToken := r.Header.Get("X-Edit-Token")
    if editToken == "" {
        editToken = r.FormValue("edit_token")
    }

    // Get image from DB
    img, err := h.db.GetImageBySlug(slug)
    if err != nil || img == nil {
        http.NotFound(w, r)
        return
    }

    // Verify edit token
    if img.EditToken == "" || img.EditToken != editToken {
        jsonError(w, "invalid edit token", http.StatusForbidden)
        return
    }

    // Delete files
    if err := h.fs.Delete(slug); err != nil {
        log.Printf("delete files error: %v", err)
    }

    // Delete from DB
    if err := h.db.DeleteImageBySlug(slug); err != nil {
        jsonError(w, "database error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"deleted": slug})
}
```

**Step 2: Dodaj routing w main.go**

W `cmd/dajtu/main.go` znajdź sekcję obsługującą `/i/` i dodaj obsługę DELETE:
```go
// W handlerze /i/{slug}
if r.Method == http.MethodDelete {
    uploadHandler.DeleteImage(w, r, slug)
    return
}
```

**Step 3: Build i test**

```bash
go build -o dajtu ./cmd/dajtu
```

---

## Task 3: Zaktualizuj template image.html

**Kontekst:** Strona podglądu obrazka musi pokazywać edit_token i przycisk delete.

**Files:**
- Modify: `internal/handler/upload.go:224` - przekazać EditToken do template
- Modify: `internal/handler/templates/image.html` - dodać UI

**Step 1: Przekaż EditToken do template**

W `ImageViewHandler.ServeHTTP` zmień dane przekazywane do template:
```go
data := map[string]interface{}{
    "Image":     img,
    "BaseURL":   baseURL,
    "CanEdit":   canEdit,
    "EditToken": editToken,  // z query param ?edit=...
    "EditMode":  editMode,   // czy token się zgadza
}
```

Logika:
```go
editToken := r.URL.Query().Get("edit")
editMode := editToken != "" && img.EditToken != "" && editToken == img.EditToken
```

**Step 2: Zaktualizuj image.html**

Dodaj sekcję z edit tokenem i przyciskiem delete:
```html
{{if .EditMode}}
<div class="edit-section">
    <div class="link-row">
        <label>Edit Token:</label>
        <input type="text" readonly value="{{.EditToken}}" onclick="this.select()">
        <button onclick="copyLink(this)">Kopiuj</button>
    </div>
    <button class="delete-btn" onclick="deleteImage()">Usuń obrazek</button>
</div>
{{else if .EditToken}}
<div class="edit-section">
    <p class="error">Nieprawidłowy edit token</p>
</div>
{{end}}
```

Dodaj JavaScript do usuwania:
```javascript
function deleteImage() {
    if (!confirm('Na pewno usunąć ten obrazek?')) return;

    const token = new URLSearchParams(location.search).get('edit');
    fetch('/i/{{.Image.Slug}}', {
        method: 'DELETE',
        headers: { 'X-Edit-Token': token }
    }).then(res => {
        if (res.ok) {
            // Flash message i redirect
            sessionStorage.setItem('flash', 'Obrazek usunięty');
            location.href = '/';
        } else {
            alert('Błąd usuwania');
        }
    });
}
```

**Step 3: Dodaj flash message na stronie głównej**

W `index.html` dodaj obsługę flash z sessionStorage:
```javascript
// Na początku <script>
const flash = sessionStorage.getItem('flash');
if (flash) {
    sessionStorage.removeItem('flash');
    const div = document.createElement('div');
    div.className = 'flash';
    div.textContent = flash;
    document.body.prepend(div);
}
```

---

## Task 4: Zaktualizuj frontend - redirect z edit tokenem

**Kontekst:** Po uploadzie system przekierowuje na `/i/{slug}`. Musi też przekazać edit_token w URL.

**Files:**
- Modify: `internal/handler/templates/index.html` - zmień redirect po uploadzie

**Step 1: Zmień redirect w index.html**

W sekcji `form.addEventListener('submit', ...)` zmień:
```javascript
if (data.url) {
    window.location.href = data.url;
} else if (data.slug) {
    // Dodaj edit_token do URL
    const editParam = data.edit_token ? `?edit=${data.edit_token}` : '';
    window.location.href = isSingleUpload
        ? `/i/${data.slug}${editParam}`
        : `/g/${data.slug}${editParam}`;
}
```

---

## Task 5: Zaktualizuj gallery.html - dodaj sekcję linków

**Kontekst:** Galeria powinna pokazywać linki do udostępnienia (jak obrazek pojedynczy).

**Files:**
- Modify: `internal/handler/templates/gallery.html` - dodać sekcję linków

**Step 1: Dodaj sekcję linków do gallery.html**

Pod tytułem, przed edit-panel:
```html
<div class="links-section">
    <div class="link-row">
        <label>Link do galerii:</label>
        <input type="text" readonly value="{{.BaseURL}}/g/{{.Slug}}" onclick="this.select()">
        <button onclick="copyLink(this)">Kopiuj</button>
    </div>
    {{if .EditMode}}
    <div class="link-row">
        <label>Edit Token:</label>
        <input type="text" readonly value="{{.EditToken}}" onclick="this.select()">
        <button onclick="copyLink(this)">Kopiuj</button>
    </div>
    <div class="link-row">
        <label>Link z edycją:</label>
        <input type="text" readonly value="{{.BaseURL}}/g/{{.Slug}}?edit={{.EditToken}}" onclick="this.select()">
        <button onclick="copyLink(this)">Kopiuj</button>
    </div>
    {{end}}
</div>
```

Dodaj funkcję copyLink (taka sama jak w image.html):
```javascript
function copyLink(btn) {
    const input = btn.previousElementSibling;
    input.select();
    navigator.clipboard.writeText(input.value);
    btn.textContent = 'Skopiowano!';
    setTimeout(() => btn.textContent = 'Kopiuj', 2000);
}
```

---

## Task 6: Logo dajtu na górze obu stron

**Kontekst:** Obie strony (image.html, gallery.html) potrzebują logo na górze jako link do strony głównej.

**Files:**
- Modify: `internal/handler/templates/image.html` - już ma header z logo
- Modify: `internal/handler/templates/gallery.html` - dodać header

**Step 1: Sprawdź image.html**

Już ma:
```html
<div class="header">
    <h1><a href="/">dajtu</a></h1>
</div>
```

**Step 2: Dodaj header do gallery.html**

Na początku `.container`:
```html
<div class="header">
    <h1 class="logo"><a href="/">dajtu</a></h1>
</div>
{{if .Title}}<h2 class="gallery-title">{{.Title}}</h2>{{end}}
```

Usuń obecne `{{if .Title}}<h1>{{.Title}}</h1>{{end}}` i zamień na h2.

Dodaj style:
```css
.header { text-align: center; margin-bottom: 20px; }
.header .logo { font-size: 2rem; font-weight: 300; }
.header .logo a { color: #fff; text-decoration: none; }
.gallery-title { font-size: 1.5rem; font-weight: 400; margin-bottom: 20px; }
```

---

## Task 7: Endpoint dodania obrazków do galerii - zwróć edit_token

**Kontekst:** Przy tworzeniu galerii response musi zawierać edit_token.

**Files:**
- Verify: `internal/handler/gallery.go` - sprawdź czy Create zwraca edit_token

**Step 1: Sprawdź GalleryHandler.Create**

Upewnij się że response zawiera `edit_token`:
```go
json.NewEncoder(w).Encode(map[string]interface{}{
    "slug":       gallery.Slug,
    "edit_token": gallery.EditToken,
})
```

---

## Task 8: Commit wszystkich zmian

**Step 1: Sprawdź status**

```bash
git status
git diff --stat
```

**Step 2: Commit**

```bash
git add internal/storage/db.go internal/handler/upload.go internal/handler/gallery.go \
        internal/handler/templates/image.html internal/handler/templates/gallery.html \
        internal/handler/templates/index.html cmd/dajtu/main.go

git commit -m "feat(view): improved image/gallery view pages

- Add edit_token to image upload response
- Add DELETE /i/{slug} endpoint for single image deletion
- Show edit_token and delete button on image view when authorized
- Add links section to gallery view
- Add dajtu logo header to gallery page
- Redirect with edit_token after upload
- Flash message on delete and redirect to home

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Podsumowanie zmian

| Plik | Zmiany |
|------|--------|
| `internal/storage/db.go` | Dodaj EditToken do Image struct |
| `internal/handler/upload.go` | Generuj edit_token, zwracaj w response, dodaj DeleteImage handler |
| `cmd/dajtu/main.go` | Routing dla DELETE /i/{slug} |
| `internal/handler/templates/image.html` | Pokaż edit_token, przycisk delete, style |
| `internal/handler/templates/gallery.html` | Header z logo, sekcja linków, style |
| `internal/handler/templates/index.html` | Redirect z edit_token, flash messages |
