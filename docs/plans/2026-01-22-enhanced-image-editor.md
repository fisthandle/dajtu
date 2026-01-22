# Enhanced Image Editor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rozbudowa edytora Cropper.js o pełny flow uploadu (single/multi), edycję istniejących obrazków dla zalogowanych użytkowników, oraz przeprojektowanie layoutu strony głównej.

**Architecture:**
- Frontend: Cropper.js 1.6.2 z większymi miniaturami (150x150), przyciskami edit/delete na każdym obrazku
- Backend: nowe endpointy `/i/{slug}/edit` (GET/POST), migracja DB dodająca pole `edited`
- Strona pojedynczego obrazka: nowy template z tabelką linków i podglądem
- Layout: minimalistyczny header, szeroki upload box (80%)

**Tech Stack:** Go, SQLite, Cropper.js, libvips, vanilla JS

---

## Task 0: Redesign layoutu strony głównej

**Files:**
- Modify: `internal/handler/templates/index.html:10-45` (CSS)
- Modify: `internal/handler/templates/index.html:257-258` (HTML header)

**Step 1: Usuń vertical centering z body**

```css
body {
    font-family: system-ui, -apple-system, sans-serif;
    background: #111;
    color: #fff;
    min-height: 100vh;
    margin: 0;
    padding: 0;
}
```

**Step 2: Zmień container na 80% szerokości**

```css
.container {
    max-width: 80%;
    width: 100%;
    margin: 0 auto;
    padding: 20px;
}
```

**Step 3: Zmień h1 - margin-top 1em, mniejszy margin-bottom**

```css
h1 {
    font-size: 2.5rem;
    font-weight: 300;
    margin-top: 1em;
    margin-bottom: 20px;
    text-align: center;
}
```

**Step 4: Usuń .subtitle z CSS (linie 29-32)**

**Step 5: Zmniejsz padding upload-area**

```css
.upload-area {
    border: 2px dashed #333;
    border-radius: 12px;
    padding: 30px 20px;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s;
}
```

**Step 6: Usuń subtitle z HTML**

```html
<h1>dajtu</h1>
<!-- USUNĄĆ: <p class="subtitle">Szybki hosting obrazków</p> -->
```

**Step 7: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "style: redesign layout - minimal header, 80% width container"
```

---

## Task 1: Większe miniatury w preview (150x150)

**Files:**
- Modify: `internal/handler/templates/index.html` (CSS .preview, .preview-item)

**Step 1: Zmień grid na 150px**

```css
.preview {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 15px;
    margin-top: 20px;
}

.preview img {
    width: 150px;
    height: 150px;
    object-fit: cover;
    border-radius: 8px;
}
```

**Step 2: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "style: increase preview thumbnails to 150x150"
```

---

## Task 2: Przyciski edit/delete na miniaturach

**Files:**
- Modify: `internal/handler/templates/index.html` (CSS + JS)

**Step 1: Dodaj CSS dla .preview-item z przyciskami overlay**

```css
.preview-item {
    position: relative;
    width: 150px;
    height: 150px;
}

.preview-item img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 8px;
}

.preview-item .btn-edit,
.preview-item .btn-delete {
    position: absolute;
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 16px;
    opacity: 0.8;
    transition: opacity 0.2s;
}

.preview-item .btn-edit {
    top: 5px;
    left: 5px;
    background: #2196F3;
    color: white;
}

.preview-item .btn-delete {
    top: 5px;
    right: 5px;
    background: #f44336;
    color: white;
}

.preview-item .btn-edit:hover,
.preview-item .btn-delete:hover {
    opacity: 1;
}
```

**Step 2: Zmień updatePreview() żeby generował .preview-item z przyciskami**

```javascript
function updatePreview() {
    const files = fileInput.files;
    previewContainer.innerHTML = '';

    Array.from(files).forEach((file, index) => {
        const item = document.createElement('div');
        item.className = 'preview-item';
        item.dataset.index = index;

        const img = document.createElement('img');
        img.src = URL.createObjectURL(file);

        const editBtn = document.createElement('button');
        editBtn.className = 'btn-edit';
        editBtn.innerHTML = '✏️';
        editBtn.title = 'Edytuj';
        editBtn.onclick = (e) => {
            e.stopPropagation();
            openEditorForFile(index);
        };

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'btn-delete';
        deleteBtn.innerHTML = '✕';
        deleteBtn.title = 'Usuń';
        deleteBtn.onclick = (e) => {
            e.stopPropagation();
            removeFile(index);
        };

        item.appendChild(img);
        item.appendChild(editBtn);
        item.appendChild(deleteBtn);
        previewContainer.appendChild(item);
    });
}
```

**Step 3: Dodaj funkcję removeFile()**

```javascript
function removeFile(index) {
    const dt = new DataTransfer();
    const files = fileInput.files;

    for (let i = 0; i < files.length; i++) {
        if (i !== index) {
            dt.items.add(files[i]);
        }
    }

    // Usuń edytowane dane dla tego pliku
    if (editedFiles[index]) {
        delete editedFiles[index];
    }
    // Przeindeksuj editedFiles
    const newEditedFiles = {};
    Object.keys(editedFiles).forEach(key => {
        const oldIndex = parseInt(key);
        if (oldIndex > index) {
            newEditedFiles[oldIndex - 1] = editedFiles[key];
        } else {
            newEditedFiles[key] = editedFiles[key];
        }
    });
    editedFiles = newEditedFiles;

    fileInput.files = dt.files;
    updatePreview();
}
```

**Step 4: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "feat: add edit/delete buttons on preview thumbnails"
```

---

## Task 3: Edytor dla wielu plików - zapisywanie do tymczasowych danych

**Files:**
- Modify: `internal/handler/templates/index.html` (JS)

**Step 1: Dodaj obiekt na edytowane pliki**

```javascript
// Na początku <script>
let editedFiles = {}; // {index: {blob: Blob, transforms: {...}}}
let currentEditIndex = null;
let originalFileData = {}; // {index: dataURL} - oryginalne pliki do ponownej edycji
```

**Step 2: Dodaj funkcję openEditorForFile()**

```javascript
function openEditorForFile(index) {
    currentEditIndex = index;
    const file = fileInput.files[index];

    // Zawsze otwieraj oryginał (nie edytowaną wersję)
    const reader = new FileReader();
    reader.onload = (e) => {
        originalFileData[index] = e.target.result;
        editorImage.src = e.target.result;
        editorContainer.style.display = 'block';

        if (cropper) {
            cropper.destroy();
        }

        cropper = new Cropper(editorImage, {
            viewMode: 1,
            dragMode: 'move',
            autoCropArea: 1,
            restore: false,
            guides: true,
            center: true,
            highlight: false,
            cropBoxMovable: true,
            cropBoxResizable: true,
            toggleDragModeOnDblclick: false,
            ready: function() {
                historyStack = [cropper.getData()];
                historyIndex = 0;
            }
        });
    };
    reader.readAsDataURL(file);
}
```

**Step 3: Zmień przycisk "Zastosuj" żeby zapisywał do editedFiles**

```javascript
document.getElementById('applyEdit').addEventListener('click', function() {
    if (!cropper || currentEditIndex === null) return;

    const canvas = cropper.getCroppedCanvas({
        maxWidth: 4096,
        maxHeight: 4096,
        imageSmoothingEnabled: true,
        imageSmoothingQuality: 'high'
    });

    canvas.toBlob((blob) => {
        editedFiles[currentEditIndex] = {
            blob: blob,
            transforms: getTransformParams()
        };

        // Aktualizuj miniaturę
        const item = previewContainer.querySelector(`[data-index="${currentEditIndex}"]`);
        if (item) {
            const img = item.querySelector('img');
            img.src = URL.createObjectURL(blob);
        }

        closeEditor();
    }, 'image/jpeg', 0.95);
});
```

**Step 4: Dodaj funkcję closeEditor()**

```javascript
function closeEditor() {
    editorContainer.style.display = 'none';
    currentEditIndex = null;
    if (cropper) {
        cropper.destroy();
        cropper = null;
    }
}

// Przycisk anuluj
document.getElementById('cancelEdit')?.addEventListener('click', closeEditor);
```

**Step 5: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "feat: editor saves to temporary data, always opens original"
```

---

## Task 4: Wysyłanie z edytowanymi plikami

**Files:**
- Modify: `internal/handler/templates/index.html` (JS - form submit)

**Step 1: Zmień obsługę submit formularza**

```javascript
uploadForm.addEventListener('submit', async function(e) {
    e.preventDefault();

    const formData = new FormData();
    const files = fileInput.files;
    const title = titleInput.value.trim();

    // Dodaj tytuł jeśli jest
    if (title) {
        formData.append('title', title);
    }

    // Dla każdego pliku - użyj edytowanego blob lub oryginału
    for (let i = 0; i < files.length; i++) {
        if (editedFiles[i] && editedFiles[i].blob) {
            // Użyj edytowanego pliku
            formData.append('files[]', editedFiles[i].blob, files[i].name);
        } else {
            // Użyj oryginału
            formData.append('files[]', files[i]);
        }
    }

    // Wybierz endpoint
    const endpoint = (files.length === 1 && !title) ? '/upload' : '/gallery';

    try {
        const response = await fetch(endpoint, {
            method: 'POST',
            body: formData
        });

        if (response.ok) {
            const result = await response.json();
            // Przekieruj do strony obrazka lub galerii
            if (result.slug) {
                window.location.href = `/i/${result.slug}`;
            } else if (result.gallery_slug) {
                window.location.href = `/g/${result.gallery_slug}`;
            }
        } else {
            alert('Błąd uploadu');
        }
    } catch (err) {
        alert('Błąd połączenia');
    }
});
```

**Step 2: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "feat: submit uses edited blobs when available"
```

---

## Task 5: Template strony pojedynczego obrazka

**Files:**
- Create: `internal/handler/templates/image.html`

**Step 1: Utwórz template**

```html
<!DOCTYPE html>
<html lang="pl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Image.OriginalName}} - dajtu</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: system-ui, -apple-system, sans-serif;
            background: #111;
            color: #fff;
            min-height: 100vh;
            padding: 1em;
        }
        .header {
            text-align: center;
            margin-bottom: 20px;
        }
        .header h1 {
            font-size: 2rem;
            font-weight: 300;
        }
        .header h1 a {
            color: #fff;
            text-decoration: none;
        }
        .links-table {
            max-width: 800px;
            margin: 0 auto 30px;
            background: #1a1a1a;
            border-radius: 12px;
            padding: 20px;
        }
        .links-table h2 {
            font-size: 1rem;
            margin-bottom: 15px;
            color: #888;
        }
        .link-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 10px 0;
            border-bottom: 1px solid #333;
        }
        .link-row:last-child {
            border-bottom: none;
        }
        .link-row label {
            color: #888;
            min-width: 100px;
        }
        .link-row input {
            flex: 1;
            background: #222;
            border: 1px solid #333;
            color: #fff;
            padding: 8px 12px;
            border-radius: 6px;
            margin: 0 10px;
            font-family: monospace;
        }
        .link-row button {
            background: #333;
            border: none;
            color: #fff;
            padding: 8px 15px;
            border-radius: 6px;
            cursor: pointer;
        }
        .link-row button:hover {
            background: #444;
        }
        .back-link {
            text-align: center;
            margin-bottom: 20px;
        }
        .back-link a {
            color: #2196F3;
            text-decoration: none;
        }
        .image-container {
            text-align: center;
        }
        .image-container img {
            max-width: 100%;
            max-height: 80vh;
            border-radius: 8px;
        }
        {{if .CanEdit}}
        .edit-btn {
            display: inline-block;
            background: #2196F3;
            color: #fff;
            padding: 10px 20px;
            border-radius: 6px;
            text-decoration: none;
            margin-bottom: 20px;
        }
        .edit-btn:hover {
            background: #1976D2;
        }
        {{end}}
    </style>
</head>
<body>
    <div class="header">
        <h1><a href="/">dajtu</a></h1>
    </div>

    <div class="links-table">
        <h2>Linki do obrazka</h2>
        <div class="link-row">
            <label>Bezpośredni:</label>
            <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/original" onclick="this.select()">
            <button onclick="copyLink(this)">Kopiuj</button>
        </div>
        <div class="link-row">
            <label>1920px:</label>
            <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/1920" onclick="this.select()">
            <button onclick="copyLink(this)">Kopiuj</button>
        </div>
        <div class="link-row">
            <label>800px:</label>
            <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/800" onclick="this.select()">
            <button onclick="copyLink(this)">Kopiuj</button>
        </div>
        <div class="link-row">
            <label>Miniatura:</label>
            <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/thumb" onclick="this.select()">
            <button onclick="copyLink(this)">Kopiuj</button>
        </div>
        <div class="link-row">
            <label>BBCode:</label>
            <input type="text" readonly value="[img]{{.BaseURL}}/i/{{.Image.Slug}}/original[/img]" onclick="this.select()">
            <button onclick="copyLink(this)">Kopiuj</button>
        </div>
        <div class="link-row">
            <label>HTML:</label>
            <input type="text" readonly value='<img src="{{.BaseURL}}/i/{{.Image.Slug}}/original">' onclick="this.select()">
            <button onclick="copyLink(this)">Kopiuj</button>
        </div>
    </div>

    <div class="back-link">
        <a href="/">← Wróć dodać kolejny</a>
        {{if .CanEdit}}
        <a href="/i/{{.Image.Slug}}/edit" class="edit-btn">✏️ Edytuj</a>
        {{end}}
    </div>

    <div class="image-container">
        <img src="/i/{{.Image.Slug}}/1920" alt="{{.Image.OriginalName}}">
    </div>

    <script>
    function copyLink(btn) {
        const input = btn.previousElementSibling;
        input.select();
        navigator.clipboard.writeText(input.value);
        btn.textContent = 'Skopiowano!';
        setTimeout(() => btn.textContent = 'Kopiuj', 2000);
    }
    </script>
</body>
</html>
```

**Step 2: Commit**

```bash
git add internal/handler/templates/image.html
git commit -m "feat: add image view template with links table"
```

---

## Task 6: Handler dla strony pojedynczego obrazka

**Files:**
- Modify: `cmd/dajtu/main.go` - routing
- Modify: `internal/handler/upload.go` - nowy handler

**Step 1: Dodaj ImageViewHandler struct w upload.go**

```go
type ImageViewHandler struct {
    db      *storage.DB
    tmpl    *template.Template
    baseURL string
}

func NewImageViewHandler(db *storage.DB, tmpl *template.Template, baseURL string) *ImageViewHandler {
    return &ImageViewHandler{db: db, tmpl: tmpl, baseURL: baseURL}
}

func (h *ImageViewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    if slug == "" {
        http.Error(w, "Not found", 404)
        return
    }

    img, err := h.db.GetImageBySlug(slug)
    if err != nil {
        http.Error(w, "Not found", 404)
        return
    }

    // Sprawdź czy użytkownik może edytować
    canEdit := false
    if userID := auth.GetUserID(r); userID != nil && img.UserID != nil && *userID == *img.UserID {
        canEdit = true
    }

    data := map[string]interface{}{
        "Image":   img,
        "BaseURL": h.baseURL,
        "CanEdit": canEdit,
    }

    h.tmpl.ExecuteTemplate(w, "image.html", data)
}
```

**Step 2: Dodaj routing w main.go**

```go
// Po załadowaniu templates
imageViewTmpl := template.Must(template.ParseFiles("internal/handler/templates/image.html"))
imageViewHandler := handler.NewImageViewHandler(db, imageViewTmpl, cfg.BaseURL)

// W routingu - PRZED /i/{slug}/{size}
r.Get("/i/{slug}", imageViewHandler.ServeHTTP)
// Dodaj też /i/{slug}/ (ze slashem)
r.Get("/i/{slug}/", imageViewHandler.ServeHTTP)
```

**Step 3: Zmień upload response żeby zwracał JSON z redirect**

W `UploadHandler.ServeHTTP`, na końcu:

```go
// Zwróć JSON z informacją o sukcesie
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]string{
    "slug": slug,
    "url":  fmt.Sprintf("/i/%s", slug),
})
```

**Step 4: Commit**

```bash
git add cmd/dajtu/main.go internal/handler/upload.go internal/handler/templates/image.html
git commit -m "feat: add image view page with links and edit option"
```

---

## Task 7: Migracja DB - pole `edited` w tabeli images

**Files:**
- Modify: `internal/storage/db.go` - migracja
- Modify: `internal/storage/db.go` - struct Image

**Step 1: Dodaj pole Edited do struct Image**

```go
type Image struct {
    ID           int64
    Slug         string
    OriginalName string
    MimeType     string
    FileSize     int64
    Width        int
    Height       int
    UserID       *int64
    CreatedAt    int64
    AccessedAt   int64
    Downloads    int64
    GalleryID    *int64
    Edited       bool  // NOWE
}
```

**Step 2: Dodaj migrację po istniejących migracjach**

```go
// Migration: add edited column if missing
_, err = db.conn.Exec(`ALTER TABLE images ADD COLUMN edited INTEGER NOT NULL DEFAULT 0`)
if err != nil && !strings.Contains(err.Error(), "duplicate column") {
    return fmt.Errorf("migrate edited: %w", err)
}
```

**Step 3: Zaktualizuj zapytania SELECT żeby pobierały `edited`**

W `GetImageBySlug`, `GetImagesByGallery`, itp:

```go
// Dodaj edited do SELECT
// Dodaj &img.Edited do Scan
```

**Step 4: Commit**

```bash
git add internal/storage/db.go
git commit -m "feat: add edited column to images table"
```

---

## Task 8: Endpoint edycji istniejącego obrazka (GET)

**Files:**
- Create: `internal/handler/templates/edit_image.html`
- Modify: `internal/handler/upload.go`

**Step 1: Utwórz template edit_image.html**

```html
<!DOCTYPE html>
<html lang="pl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Edycja - {{.Image.OriginalName}}</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/cropperjs/1.6.2/cropper.min.css">
    <style>
        /* ... styl edytora jak w index.html ... */
        body {
            font-family: system-ui, -apple-system, sans-serif;
            background: #111;
            color: #fff;
            min-height: 100vh;
            padding: 1em;
        }
        .header { text-align: center; margin-bottom: 20px; }
        .header h1 { font-size: 2rem; font-weight: 300; }
        .header h1 a { color: #fff; text-decoration: none; }

        .editor-container {
            max-width: 90%;
            margin: 0 auto;
        }
        .editor-image-wrapper {
            max-height: 60vh;
            overflow: hidden;
        }
        .editor-image-wrapper img {
            max-width: 100%;
            display: block;
        }
        .toolbar {
            display: flex;
            flex-wrap: wrap;
            gap: 10px;
            justify-content: center;
            margin: 20px 0;
            padding: 15px;
            background: #1a1a1a;
            border-radius: 8px;
        }
        .toolbar button {
            background: #333;
            border: none;
            color: #fff;
            padding: 10px 15px;
            border-radius: 6px;
            cursor: pointer;
        }
        .toolbar button:hover { background: #444; }
        .toolbar button.active { background: #2196F3; }

        .save-options {
            max-width: 600px;
            margin: 20px auto;
            padding: 20px;
            background: #1a1a1a;
            border-radius: 8px;
        }
        .save-options h3 {
            margin-bottom: 15px;
            color: #888;
        }
        .save-options label {
            display: block;
            padding: 10px;
            margin: 5px 0;
            background: #222;
            border-radius: 6px;
            cursor: pointer;
        }
        .save-options label:hover { background: #333; }
        .save-options input[type="radio"] { margin-right: 10px; }

        .action-buttons {
            display: flex;
            gap: 15px;
            justify-content: center;
            margin-top: 20px;
        }
        .btn-save {
            background: #4CAF50;
            color: white;
            border: none;
            padding: 12px 30px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 1rem;
        }
        .btn-cancel {
            background: #666;
            color: white;
            border: none;
            padding: 12px 30px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 1rem;
            text-decoration: none;
        }
        {{if .Image.Edited}}
        .btn-restore {
            background: #FF9800;
            color: white;
            border: none;
            padding: 12px 30px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 1rem;
        }
        {{end}}
    </style>
</head>
<body>
    <div class="header">
        <h1><a href="/">dajtu</a></h1>
    </div>

    <div class="editor-container">
        <div class="editor-image-wrapper">
            <img id="editorImage" src="/i/{{.Image.Slug}}/original">
        </div>

        <div class="toolbar">
            <button onclick="cropper.rotate(-90)" title="Obróć w lewo">↺</button>
            <button onclick="cropper.rotate(90)" title="Obróć w prawo">↻</button>
            <button onclick="cropper.scaleX(-cropper.getData().scaleX || -1)" title="Odbij poziomo">⇆</button>
            <button onclick="cropper.scaleY(-cropper.getData().scaleY || -1)" title="Odbij pionowo">⇅</button>
            <span style="width: 20px"></span>
            <button onclick="setAspectRatio(1)" title="1:1">1:1</button>
            <button onclick="setAspectRatio(16/9)" title="16:9">16:9</button>
            <button onclick="setAspectRatio(4/3)" title="4:3">4:3</button>
            <button onclick="setAspectRatio(NaN)" title="Dowolny">Free</button>
            <span style="width: 20px"></span>
            <button onclick="undo()" title="Cofnij">↩</button>
            <button onclick="redo()" title="Ponów">↪</button>
            <button onclick="cropper.reset()" title="Reset">⟲</button>
        </div>

        <div class="save-options">
            <h3>Zapisz jako:</h3>
            <label>
                <input type="radio" name="saveMode" value="replace" checked>
                Zastąp istniejący plik
            </label>
            <label>
                <input type="radio" name="saveMode" value="new">
                Utwórz nowy plik (zachowaj oryginał)
            </label>
        </div>

        <div class="action-buttons">
            <a href="/i/{{.Image.Slug}}" class="btn-cancel">Anuluj</a>
            {{if .Image.Edited}}
            <button class="btn-restore" onclick="restoreOriginal()">Przywróć oryginał</button>
            {{end}}
            <button class="btn-save" onclick="saveEdit()">Zapisz</button>
        </div>
    </div>

    <script src="https://cdnjs.cloudflare.com/ajax/libs/cropperjs/1.6.2/cropper.min.js"></script>
    <script>
    const slug = '{{.Image.Slug}}';
    let cropper;
    let historyStack = [];
    let historyIndex = -1;

    const img = document.getElementById('editorImage');
    img.onload = function() {
        cropper = new Cropper(img, {
            viewMode: 1,
            dragMode: 'move',
            autoCropArea: 1,
            restore: false,
            guides: true,
            center: true,
            highlight: false,
            cropBoxMovable: true,
            cropBoxResizable: true,
            toggleDragModeOnDblclick: false,
            ready: function() {
                historyStack = [cropper.getData()];
                historyIndex = 0;
            },
            cropend: saveHistory
        });
    };

    function saveHistory() {
        historyStack = historyStack.slice(0, historyIndex + 1);
        historyStack.push(cropper.getData());
        historyIndex = historyStack.length - 1;
    }

    function undo() {
        if (historyIndex > 0) {
            historyIndex--;
            cropper.setData(historyStack[historyIndex]);
        }
    }

    function redo() {
        if (historyIndex < historyStack.length - 1) {
            historyIndex++;
            cropper.setData(historyStack[historyIndex]);
        }
    }

    function setAspectRatio(ratio) {
        cropper.setAspectRatio(ratio);
        saveHistory();
    }

    async function saveEdit() {
        const saveMode = document.querySelector('input[name="saveMode"]:checked').value;
        const canvas = cropper.getCroppedCanvas({
            maxWidth: 4096,
            maxHeight: 4096,
            imageSmoothingEnabled: true,
            imageSmoothingQuality: 'high'
        });

        canvas.toBlob(async (blob) => {
            const formData = new FormData();
            formData.append('file', blob, 'edited.jpg');
            formData.append('mode', saveMode);

            try {
                const response = await fetch(`/i/${slug}/edit`, {
                    method: 'POST',
                    body: formData
                });

                if (response.ok) {
                    const result = await response.json();
                    window.location.href = `/i/${result.slug}`;
                } else {
                    alert('Błąd zapisu');
                }
            } catch (err) {
                alert('Błąd połączenia');
            }
        }, 'image/jpeg', 0.95);
    }

    async function restoreOriginal() {
        if (!confirm('Czy na pewno chcesz przywrócić oryginalny plik?')) return;

        try {
            const response = await fetch(`/i/${slug}/restore`, {
                method: 'POST'
            });

            if (response.ok) {
                window.location.href = `/i/${slug}`;
            } else {
                alert('Błąd przywracania');
            }
        } catch (err) {
            alert('Błąd połączenia');
        }
    }
    </script>
</body>
</html>
```

**Step 2: Commit**

```bash
git add internal/handler/templates/edit_image.html
git commit -m "feat: add edit image template with cropper.js"
```

---

## Task 9: Handler edycji obrazka (POST)

**Files:**
- Modify: `internal/handler/upload.go`
- Modify: `cmd/dajtu/main.go`

**Step 1: Dodaj ImageEditHandler**

```go
type ImageEditHandler struct {
    db        *storage.DB
    fs        *storage.FileSystem
    tmpl      *template.Template
    processor *image.Processor
    cfg       *config.Config
}

func NewImageEditHandler(db *storage.DB, fs *storage.FileSystem, tmpl *template.Template, processor *image.Processor, cfg *config.Config) *ImageEditHandler {
    return &ImageEditHandler{db: db, fs: fs, tmpl: tmpl, processor: processor, cfg: cfg}
}

func (h *ImageEditHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")

    img, err := h.db.GetImageBySlug(slug)
    if err != nil {
        http.Error(w, "Not found", 404)
        return
    }

    // Sprawdź uprawnienia
    userID := auth.GetUserID(r)
    if userID == nil || img.UserID == nil || *userID != *img.UserID {
        http.Error(w, "Forbidden", 403)
        return
    }

    if r.Method == "GET" {
        // Pokaż formularz edycji
        h.tmpl.ExecuteTemplate(w, "edit_image.html", map[string]interface{}{
            "Image": img,
        })
        return
    }

    // POST - zapisz edycję
    file, _, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "No file", 400)
        return
    }
    defer file.Close()

    data, err := io.ReadAll(file)
    if err != nil {
        http.Error(w, "Read error", 500)
        return
    }

    mode := r.FormValue("mode") // "replace" lub "new"

    if mode == "new" {
        // Utwórz nowy obrazek
        newSlug := generateSlug(5)

        sizes, err := h.processor.Process(data)
        if err != nil {
            http.Error(w, "Process error", 500)
            return
        }

        for size, sizeData := range sizes {
            if err := h.fs.Save(newSlug, size, sizeData); err != nil {
                http.Error(w, "Save error", 500)
                return
            }
        }

        newImg := &storage.Image{
            Slug:         newSlug,
            OriginalName: img.OriginalName,
            MimeType:     "image/webp",
            FileSize:     int64(len(data)),
            Width:        sizes["original"].Width,
            Height:       sizes["original"].Height,
            UserID:       userID,
            CreatedAt:    time.Now().Unix(),
            AccessedAt:   time.Now().Unix(),
            Edited:       true,
            GalleryID:    img.GalleryID,
        }

        if err := h.db.CreateImage(newImg); err != nil {
            http.Error(w, "DB error", 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"slug": newSlug})
        return
    }

    // mode == "replace" - zastąp istniejący
    sizes, err := h.processor.Process(data)
    if err != nil {
        http.Error(w, "Process error", 500)
        return
    }

    // Zapisz backup oryginału jeśli jeszcze nie był edytowany
    if !img.Edited {
        // Skopiuj current original do backup
        // ... logika backupu ...
    }

    // Zastąp wszystkie rozmiary
    for size, sizeData := range sizes {
        if err := h.fs.Save(slug, size, sizeData); err != nil {
            http.Error(w, "Save error", 500)
            return
        }
    }

    // Oznacz jako edytowany
    if err := h.db.MarkImageEdited(slug); err != nil {
        http.Error(w, "DB error", 500)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"slug": slug})
}
```

**Step 2: Dodaj MarkImageEdited do db.go**

```go
func (db *DB) MarkImageEdited(slug string) error {
    _, err := db.conn.Exec("UPDATE images SET edited = 1 WHERE slug = ?", slug)
    return err
}
```

**Step 3: Dodaj routing**

```go
r.Get("/i/{slug}/edit", imageEditHandler.ServeHTTP)
r.Post("/i/{slug}/edit", imageEditHandler.ServeHTTP)
```

**Step 4: Commit**

```bash
git add internal/handler/upload.go internal/storage/db.go cmd/dajtu/main.go
git commit -m "feat: add image edit endpoint with replace/new modes"
```

---

## Task 10: Endpoint przywracania oryginału

**Files:**
- Modify: `internal/handler/upload.go`
- Modify: `internal/storage/filesystem.go`

**Step 1: Dodaj przechowywanie backup przy pierwszej edycji**

W FileSystem dodaj:

```go
func (fs *FileSystem) SaveBackup(slug string) error {
    // Skopiuj /slug/original do /slug/backup
    src := filepath.Join(fs.basePath, slug, "original.webp")
    dst := filepath.Join(fs.basePath, slug, "backup.webp")

    data, err := os.ReadFile(src)
    if err != nil {
        return err
    }
    return os.WriteFile(dst, data, 0644)
}

func (fs *FileSystem) RestoreFromBackup(slug string) error {
    // Skopiuj /slug/backup do /slug/original i przetworz ponownie
    src := filepath.Join(fs.basePath, slug, "backup.webp")
    dst := filepath.Join(fs.basePath, slug, "original.webp")

    data, err := os.ReadFile(src)
    if err != nil {
        return err
    }
    return os.WriteFile(dst, data, 0644)
}

func (fs *FileSystem) HasBackup(slug string) bool {
    path := filepath.Join(fs.basePath, slug, "backup.webp")
    _, err := os.Stat(path)
    return err == nil
}
```

**Step 2: Dodaj RestoreHandler**

```go
func (h *ImageEditHandler) RestoreOriginal(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")

    img, err := h.db.GetImageBySlug(slug)
    if err != nil || !img.Edited {
        http.Error(w, "Not found", 404)
        return
    }

    // Sprawdź uprawnienia
    userID := auth.GetUserID(r)
    if userID == nil || img.UserID == nil || *userID != *img.UserID {
        http.Error(w, "Forbidden", 403)
        return
    }

    // Przywróć z backupu
    if err := h.fs.RestoreFromBackup(slug); err != nil {
        http.Error(w, "Restore error", 500)
        return
    }

    // Przetworz ponownie wszystkie rozmiary
    backupData, _ := h.fs.ReadBackup(slug)
    sizes, _ := h.processor.Process(backupData)
    for size, sizeData := range sizes {
        h.fs.Save(slug, size, sizeData)
    }

    // Usuń flagę edited
    h.db.UnmarkImageEdited(slug)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

**Step 3: Dodaj routing**

```go
r.Post("/i/{slug}/restore", imageEditHandler.RestoreOriginal)
```

**Step 4: Commit**

```bash
git add internal/handler/upload.go internal/storage/filesystem.go internal/storage/db.go cmd/dajtu/main.go
git commit -m "feat: add restore from original functionality"
```

---

## Task 11: Wskaźnik "edytowany" w admin panel

**Files:**
- Modify: `internal/handler/templates/admin/images.html`

**Step 1: Dodaj kolumnę "Edited" w tabeli**

```html
<th>Edited</th>
<!-- ... -->
<td>{{if .Edited}}✏️{{end}}</td>
```

**Step 2: Commit**

```bash
git add internal/handler/templates/admin/images.html
git commit -m "feat: show edited indicator in admin panel"
```

---

## Podsumowanie

| Task | Opis | Pliki |
|------|------|-------|
| 0 | Redesign layoutu | index.html |
| 1 | Większe miniatury 150x150 | index.html |
| 2 | Przyciski edit/delete | index.html |
| 3 | Edytor dla wielu plików | index.html |
| 4 | Wysyłanie z edytowanymi | index.html |
| 5 | Template strony obrazka | image.html |
| 6 | Handler strony obrazka | upload.go, main.go |
| 7 | Migracja DB - pole edited | db.go |
| 8 | Template edycji obrazka | edit_image.html |
| 9 | Handler edycji (GET/POST) | upload.go, main.go |
| 10 | Przywracanie oryginału | upload.go, filesystem.go |
| 11 | Wskaźnik w admin | admin/images.html |
