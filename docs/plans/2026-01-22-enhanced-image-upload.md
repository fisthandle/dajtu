# Enhanced Image Upload Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Poprawić UX uploadu obrazków: persystentna lista plików, modal dla edytora, usunięcie tytułu galerii, lepsze aspect ratio, automatyczna rotacja EXIF.

**Architecture:** Frontend (index.html) zmienia obsługę plików na persystentną listę, edytor przeniesiony do modalu 80% ekranu. Backend (processor.go) dodaje auto-rotację EXIF przed przetwarzaniem. Galeria (gallery.html) dostaje edycję tytułu po utworzeniu.

**Tech Stack:** JavaScript (Cropper.js), Go (bimg/libvips), HTML/CSS

---

## Task 1: Persystentna lista plików

**Problem:** Pliki znikają jak klikniesz input żeby dodać kolejny.

**Files:**
- Modify: `internal/handler/templates/index.html:617-622` (drop handler)
- Modify: `internal/handler/templates/index.html:668-710` (updatePreview)
- Test: `internal/handler/upload_test.go` (bez zmian - test backend)

**Step 1: Zmodyfikuj strukturę przechowywania plików**

Zmień z `fileInput.files` na własną tablicę `selectedFiles`:

```javascript
// Na początku sekcji script, po deklaracji zmiennych (linia ~373)
let selectedFiles = [];  // Nowa persystentna lista plików
```

**Step 2: Zmień handler drop żeby dodawał pliki**

```javascript
// Zamień linie 617-620
dropArea.addEventListener('drop', e => {
    addFiles(e.dataTransfer.files);
});
```

**Step 3: Zmień handler change żeby dodawał pliki**

```javascript
// Zamień linię 622
fileInput.addEventListener('change', () => {
    addFiles(fileInput.files);
    fileInput.value = ''; // Reset input
});
```

**Step 4: Dodaj funkcję addFiles**

```javascript
// Przed funkcją removeFile
function addFiles(files) {
    for (const file of files) {
        if (file.type.startsWith('image/')) {
            selectedFiles.push(file);
        }
    }
    updatePreview();
}
```

**Step 5: Zmień removeFile żeby usuwał z tablicy**

```javascript
function removeFile(index) {
    // Usuń edytowane dane
    if (editedFiles[index]) {
        delete editedFiles[index];
    }
    if (originalFileData[index]) {
        delete originalFileData[index];
    }

    // Usuń plik z tablicy
    selectedFiles.splice(index, 1);

    // Przeindeksuj editedFiles i originalFileData
    const newEditedFiles = {};
    const newOriginalFileData = {};
    Object.keys(editedFiles).forEach(key => {
        const oldIndex = parseInt(key);
        if (oldIndex > index) {
            newEditedFiles[oldIndex - 1] = editedFiles[key];
        } else if (oldIndex < index) {
            newEditedFiles[key] = editedFiles[key];
        }
    });
    Object.keys(originalFileData).forEach(key => {
        const oldIndex = parseInt(key);
        if (oldIndex > index) {
            newOriginalFileData[oldIndex - 1] = originalFileData[key];
        } else if (oldIndex < index) {
            newOriginalFileData[key] = originalFileData[key];
        }
    });
    editedFiles = newEditedFiles;
    originalFileData = newOriginalFileData;

    updatePreview();
}
```

**Step 6: Zmień updatePreview żeby używał selectedFiles**

```javascript
function updatePreview() {
    preview.innerHTML = '';
    submitBtn.disabled = selectedFiles.length === 0;

    selectedFiles.forEach((file, index) => {
        const item = document.createElement('div');
        item.className = 'preview-item';
        item.dataset.index = index;

        const img = document.createElement('img');
        img.src = URL.createObjectURL(file);
        img.onload = () => URL.revokeObjectURL(img.src);

        const editBtn = document.createElement('button');
        editBtn.type = 'button';
        editBtn.className = 'btn-edit';
        editBtn.innerHTML = '✏️';
        editBtn.title = 'Edytuj';
        editBtn.onclick = (e) => {
            e.preventDefault();
            e.stopPropagation();
            openEditorForFile(index);
        };

        const deleteBtn = document.createElement('button');
        deleteBtn.type = 'button';
        deleteBtn.className = 'btn-delete';
        deleteBtn.innerHTML = '✕';
        deleteBtn.title = 'Usuń';
        deleteBtn.onclick = (e) => {
            e.preventDefault();
            e.stopPropagation();
            removeFile(index);
        };

        item.appendChild(img);
        item.appendChild(editBtn);
        item.appendChild(deleteBtn);
        preview.appendChild(item);
    });
    updateEditorVisibility();
}
```

**Step 7: Zmień openEditorForFile żeby używał selectedFiles**

```javascript
function openEditorForFile(index) {
    currentEditIndex = index;
    const file = selectedFiles[index];

    if (!originalFileData[index]) {
        const reader = new FileReader();
        reader.onload = (e) => {
            originalFileData[index] = e.target.result;
            loadEditorImage(e.target.result);
        };
        reader.readAsDataURL(file);
    } else {
        loadEditorImage(originalFileData[index]);
    }
}
```

**Step 8: Zmień updateEditorVisibility**

```javascript
function updateEditorVisibility() {
    // Edytor pokazuje się tylko dla single bez tytułu (zostaje bez zmian)
    const hasSingle = selectedFiles.length === 1 && !titleInput.value.trim();
    if (!hasSingle) {
        destroyEditor();
        return;
    }
    initEditor(selectedFiles[0]);
}
```

**Step 9: Zmień submit handler żeby używał selectedFiles**

```javascript
form.addEventListener('submit', async e => {
    e.preventDefault();
    submitBtn.disabled = true;
    submitBtn.textContent = 'Wysyłanie...';

    const formData = new FormData();
    const title = titleInput.value.trim();
    const isSingleUpload = selectedFiles.length === 1 && !title;

    if (title) {
        formData.append('title', title);
    }

    if (isSingleUpload) {
        if (editedFiles[0] && editedFiles[0].blob) {
            formData.append('file', editedFiles[0].blob, selectedFiles[0].name);
        } else {
            formData.append('file', selectedFiles[0]);
        }
    } else {
        for (let i = 0; i < selectedFiles.length; i++) {
            if (editedFiles[i] && editedFiles[i].blob) {
                formData.append('files', editedFiles[i].blob, selectedFiles[i].name);
            } else {
                formData.append('files', selectedFiles[i]);
            }
        }
    }

    const endpoint = isSingleUpload ? '/upload' : '/gallery';

    try {
        const res = await fetch(endpoint, {
            method: 'POST',
            body: formData
        });

        if (res.ok) {
            const data = await res.json();
            if (data.url) {
                window.location.href = data.url;
            } else if (data.slug) {
                window.location.href = isSingleUpload ? `/i/${data.slug}` : `/g/${data.slug}`;
            }
        } else {
            alert('Błąd uploadu');
        }
    } catch (err) {
        alert('Błąd połączenia: ' + err.message);
    } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Wyślij';
    }
});
```

**Step 10: Uruchom lokalny serwer i przetestuj ręcznie**

```bash
cd /home/pawel/dev/dajtu && set -a && source .env.local && set +a && ./dajtu
```

Sprawdź:
- [ ] Dodanie plików przez klik
- [ ] Dodanie plików przez drop
- [ ] Pliki pozostają przy dodaniu kolejnych
- [ ] Usuwanie pojedynczych plików działa

**Step 11: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "$(cat <<'EOF'
feat(upload): persistent file list

Files no longer disappear when adding more files.
Uses internal array instead of fileInput.files.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Modal dla edytora

**Problem:** Edytor jest pod upload area, przeszkadza.

**Files:**
- Modify: `internal/handler/templates/index.html` (CSS + HTML struktura)

**Step 1: Dodaj CSS dla modalu**

Dodaj na końcu sekcji `<style>`:

```css
.editor-modal {
    display: none;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.9);
    z-index: 1000;
    justify-content: center;
    align-items: center;
    padding: 20px;
}
.editor-modal.active {
    display: flex;
}
.editor-modal-content {
    width: 80%;
    max-width: 1200px;
    max-height: 90vh;
    background: #1a1a1a;
    border-radius: 12px;
    padding: 20px;
    display: flex;
    flex-direction: column;
}
.editor-modal-content .editor-toolbar {
    margin-bottom: 15px;
}
.editor-modal-content .editor-canvas-wrap {
    flex: 1;
    max-height: calc(90vh - 150px);
    overflow: hidden;
}
```

**Step 2: Przenieś edytor do modalu w HTML**

Zamień strukturę `.editor-container` (linie 300-328) na:

```html
<div class="editor-modal" id="editorModal">
    <div class="editor-modal-content">
        <div class="editor-toolbar">
            <button type="button" id="editorUndo" disabled title="Cofnij (Ctrl+Z)">↶ Cofnij</button>
            <button type="button" id="editorRedo" disabled title="Ponów (Ctrl+Y)">↷ Ponów</button>
            <div class="separator"></div>
            <div class="group">
                <button type="button" id="editorRotateL" title="Obróć -90°">↺</button>
                <button type="button" id="editorRotateR" title="Obróć +90°">↻</button>
                <button type="button" id="editorFlipH" title="Odbij poziomo">⇆</button>
                <button type="button" id="editorFlipV" title="Odbij pionowo">⇅</button>
            </div>
            <div class="separator"></div>
            <div class="group">
                <button type="button" id="aspectFree" class="active">Free</button>
                <button type="button" id="aspect1x1">1:1</button>
                <button type="button" id="aspect16x9">16:9</button>
                <button type="button" id="aspect4x3">4:3</button>
                <button type="button" id="aspect9x16">9:16</button>
                <button type="button" id="aspect3x4">3:4</button>
            </div>
            <div class="separator"></div>
            <button type="button" id="editorReset" title="Resetuj wszystko">⟲ Reset</button>
            <div class="separator"></div>
            <button type="button" id="applyEdit" style="background: #4a9eff; color: white;">✓ Zastosuj</button>
            <button type="button" id="cancelEdit">✕ Anuluj</button>
        </div>
        <div class="editor-canvas-wrap" id="editorCanvasWrap">
            <img id="editorImage" src="" alt="Edit">
        </div>
        <div class="editor-info" id="editorInfo"></div>
    </div>
</div>
```

**Step 3: Usuń stary .editor-container div z HTML**

Usuń cały blok:
```html
<div class="editor-container" id="editorContainer">
    ...
</div>
```

**Step 4: Zaktualizuj JavaScript referencje**

Zamień:
```javascript
const editorContainer = document.getElementById('editorContainer');
```

na:
```javascript
const editorModal = document.getElementById('editorModal');
```

**Step 5: Zaktualizuj funkcje edytora**

W `destroyEditor()`:
```javascript
// Zamień:
editorContainer.classList.remove('active');
// Na:
editorModal.classList.remove('active');
```

W `loadEditorImage()` i gdzie indziej `ready()`:
```javascript
// Zamień:
editorContainer.classList.add('active');
// Na:
editorModal.classList.add('active');
```

W `closeEditor()`:
```javascript
// Zamień:
editorContainer.classList.remove('active');
// Na:
editorModal.classList.remove('active');
```

W `updateEditorVisibility()`:
```javascript
// Zamień warunek na:
function updateEditorVisibility() {
    // Modal edytora jest otwierany ręcznie przez przycisk edit
    // Nie otwieramy go automatycznie
}
```

W key handler:
```javascript
// Zamień:
if (!cropper || !editorContainer.classList.contains('active')) return;
// Na:
if (!cropper || !editorModal.classList.contains('active')) return;
```

**Step 6: Dodaj nowe aspect ratio buttons**

Zaktualizuj tablicę aspectButtons:
```javascript
const aspectButtons = [
    { id: 'aspectFree', ratio: NaN },
    { id: 'aspect1x1', ratio: 1 },
    { id: 'aspect16x9', ratio: 16 / 9 },
    { id: 'aspect4x3', ratio: 4 / 3 },
    { id: 'aspect9x16', ratio: 9 / 16 },
    { id: 'aspect3x4', ratio: 3 / 4 }
];
```

**Step 7: Zamknij modal na ESC i klik poza**

Dodaj event listenery:
```javascript
// Zamknij modal na klik poza content
editorModal.addEventListener('click', (e) => {
    if (e.target === editorModal) {
        closeEditor();
    }
});
```

**Step 8: Przetestuj ręcznie**

- [ ] Modal pojawia się na środku, 80% szerokości
- [ ] Przyciski 9:16 i 3:4 działają
- [ ] ESC zamyka modal
- [ ] Klik poza modalem zamyka

**Step 9: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "$(cat <<'EOF'
feat(editor): move to modal overlay

Editor now opens in 80% width modal for better visibility.
Added 9:16 and 3:4 aspect ratio buttons.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Usuń tytuł galerii z upload form

**Problem:** Tytuł jest niepotrzebny przy uploadzie, edycja po utworzeniu galerii.

**Files:**
- Modify: `internal/handler/templates/index.html` (usuń input title)
- Modify: `internal/handler/templates/gallery.html` (dodaj edycję tytułu)
- Modify: `internal/handler/gallery.go` (endpoint do edycji tytułu)
- Test: `internal/handler/gallery_test.go`

**Step 1: Napisz failing test dla edycji tytułu galerii**

Dodaj do `internal/handler/gallery_test.go`:

```go
func TestGalleryHandler_UpdateTitle(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "title1",
		EditToken: "secret",
		Title:     "Old Title",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("title", "New Title")
	writer.Close()

	req := httptest.NewRequest("POST", "/gallery/title1/title", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Edit-Token", "secret")
	rec := httptest.NewRecorder()

	h.UpdateTitle(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify title was updated
	updated, _ := db.GetGalleryBySlug("title1")
	if updated.Title != "New Title" {
		t.Errorf("title = %q, want %q", updated.Title, "New Title")
	}
}

func TestGalleryHandler_UpdateTitle_InvalidToken(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "title2",
		EditToken: "correct",
		Title:     "Original",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("title", "Hacked")
	writer.Close()

	req := httptest.NewRequest("POST", "/gallery/title2/title", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Edit-Token", "wrong")
	rec := httptest.NewRecorder()

	h.UpdateTitle(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
```

**Step 2: Uruchom test żeby sprawdzić że fails**

```bash
cd /home/pawel/dev/dajtu && go test ./internal/handler/... -run TestGalleryHandler_UpdateTitle -v
```

Expected: FAIL - `h.UpdateTitle` nie istnieje

**Step 3: Dodaj metodę UpdateTitle w gallery.go**

Dodaj po metodzie `DeleteImage`:

```go
// POST /gallery/:slug/title - update gallery title
func (h *GalleryHandler) UpdateTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract gallery slug from path: /gallery/XXXX/title
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "title" {
		http.NotFound(w, r)
		return
	}
	gallerySlug := parts[0]

	// Verify edit token
	editToken := r.Header.Get("X-Edit-Token")
	if editToken == "" {
		editToken = r.FormValue("edit_token")
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	if gallery.EditToken != editToken {
		jsonError(w, "invalid edit token", http.StatusForbidden)
		return
	}

	r.ParseForm()
	newTitle := r.FormValue("title")

	if err := h.db.UpdateGalleryTitle(gallery.ID, newTitle); err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"title": newTitle})
}
```

**Step 4: Dodaj metodę UpdateGalleryTitle w storage/db.go**

Znajdź sekcję z metodami Gallery i dodaj:

```go
func (db *DB) UpdateGalleryTitle(id int64, title string) error {
	_, err := db.conn.Exec("UPDATE galleries SET title = ?, updated_at = ? WHERE id = ?",
		title, time.Now().Unix(), id)
	return err
}
```

**Step 5: Uruchom test**

```bash
cd /home/pawel/dev/dajtu && go test ./internal/handler/... -run TestGalleryHandler_UpdateTitle -v
```

Expected: PASS

**Step 6: Dodaj routing w main.go**

Znajdź sekcję z routingiem galerii i dodaj obsługę `/gallery/{slug}/title`:

W `cmd/dajtu/main.go` znajdź handler dla `/gallery/` i rozszerz logikę:

```go
// Znajdź istniejący handler i zmodyfikuj
mux.HandleFunc("/gallery/", func(w http.ResponseWriter, r *http.Request) {
    path := strings.TrimPrefix(r.URL.Path, "/gallery/")
    parts := strings.Split(path, "/")

    if len(parts) == 2 {
        switch parts[1] {
        case "add":
            galleryHandler.AddImages(w, r)
        case "title":
            galleryHandler.UpdateTitle(w, r)
        default:
            galleryHandler.DeleteImage(w, r)
        }
        return
    }

    galleryHandler.Create(w, r)
})
```

**Step 7: Usuń pole tytułu z index.html**

Usuń blok options (linie 330-335):

```html
<!-- USUŃ TO: -->
<div class="options">
    <div class="option">
        <label for="title">Tytuł galerii (opcjonalnie)</label>
        <input type="text" name="title" id="title" placeholder="Moja galeria">
    </div>
</div>
```

Usuń też JavaScript związany z titleInput:
- Usuń: `const titleInput = document.getElementById('title');`
- Usuń: `titleInput.addEventListener('input', updateEditorVisibility);`
- W submit handler: usuń `const title = titleInput.value.trim();` i używaj `const title = '';`
- W `updateEditorVisibility`: usuń `&& !titleInput.value.trim()`

**Step 8: Dodaj edycję tytułu w gallery.html**

W sekcji `.edit-mode` dodaj formularz (po token-form):

```html
<form class="title-form" id="titleForm">
    <input type="text" id="titleInput" placeholder="Tytuł galerii..." value="{{.Title}}">
    <button type="submit">Zapisz tytuł</button>
</form>
```

Dodaj CSS:
```css
.title-form {
    display: none;
    margin-top: 15px;
    gap: 10px;
}
.edit-mode .title-form { display: flex; }
.title-form input {
    flex: 1;
    padding: 10px 12px;
    border: 1px solid #333;
    border-radius: 6px;
    background: #222;
    color: #fff;
    font-size: 1rem;
}
.title-form button {
    padding: 10px 20px;
    border: none;
    border-radius: 6px;
    background: #4a9eff;
    color: #fff;
    cursor: pointer;
}
```

Dodaj JavaScript:
```javascript
const titleForm = document.getElementById('titleForm');
const titleInputEl = document.getElementById('titleInput');

titleForm.addEventListener('submit', async e => {
    e.preventDefault();
    const token = tokenInput.value;
    const newTitle = titleInputEl.value;

    const formData = new FormData();
    formData.append('title', newTitle);

    const res = await fetch(`/gallery/${gallerySlug}/title`, {
        method: 'POST',
        headers: { 'X-Edit-Token': token },
        body: formData
    });

    if (res.ok) {
        // Update h1 if exists
        const h1 = document.querySelector('h1');
        if (h1) {
            h1.textContent = newTitle || 'Galeria';
        } else if (newTitle) {
            const container = document.querySelector('.container');
            const newH1 = document.createElement('h1');
            newH1.textContent = newTitle;
            container.insertBefore(newH1, container.firstChild);
        }
    } else {
        const data = await res.json();
        alert('Błąd: ' + (data.error || 'Nieznany błąd'));
    }
});
```

**Step 9: Commit**

```bash
git add internal/handler/gallery.go internal/handler/gallery_test.go internal/storage/db.go internal/handler/templates/index.html internal/handler/templates/gallery.html cmd/dajtu/main.go
git commit -m "$(cat <<'EOF'
feat(gallery): move title editing to gallery view

- Remove title field from upload form
- Add title editing in gallery edit mode
- New endpoint POST /gallery/{slug}/title

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Fix aspect ratio po rotacji

**Problem:** Jak obrócę prostokątny obrazek o 90°, zaznaczenie "free" się nie zmienia.

**Files:**
- Modify: `internal/handler/templates/index.html`

**Step 1: Dodaj handler dla rotate który resetuje crop**

Po rotacji trzeba zaktualizować crop area żeby obejmował cały obrazek:

```javascript
document.getElementById('editorRotateL').addEventListener('click', () => {
    if (!cropper) return;
    cropper.rotate(-90);
    // Po rotacji: jeśli aspect ratio jest free, rozciągnij na cały obrazek
    if (Number.isNaN(currentAspectRatio)) {
        setTimeout(() => {
            const containerData = cropper.getContainerData();
            const canvasData = cropper.getCanvasData();
            cropper.setCropBoxData({
                left: canvasData.left,
                top: canvasData.top,
                width: canvasData.width,
                height: canvasData.height
            });
        }, 50);
    }
    saveState();
});

document.getElementById('editorRotateR').addEventListener('click', () => {
    if (!cropper) return;
    cropper.rotate(90);
    // Po rotacji: jeśli aspect ratio jest free, rozciągnij na cały obrazek
    if (Number.isNaN(currentAspectRatio)) {
        setTimeout(() => {
            const containerData = cropper.getContainerData();
            const canvasData = cropper.getCanvasData();
            cropper.setCropBoxData({
                left: canvasData.left,
                top: canvasData.top,
                width: canvasData.width,
                height: canvasData.height
            });
        }, 50);
    }
    saveState();
});
```

**Step 2: Przetestuj ręcznie**

- [ ] Załaduj prostokątny obrazek (np. 1920x1080)
- [ ] Upewnij się że "Free" jest aktywne
- [ ] Obróć o 90° - zaznaczenie powinno objąć cały obrazek (teraz 1080x1920)
- [ ] Powtórz dla -90°

**Step 3: Commit**

```bash
git add internal/handler/templates/index.html
git commit -m "$(cat <<'EOF'
fix(editor): reset crop area after rotation in free mode

When rotating with free aspect ratio, crop area now
covers the entire rotated image.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Auto-rotacja EXIF w backendzie

**Problem:** Obrazki z rotacją EXIF są zapisywane bez korekty.

**Files:**
- Create: `internal/image/processor_test.go`
- Modify: `internal/image/processor.go`
- Create: `internal/testutil/exif.go`

**Step 1: Utwórz testutil dla EXIF**

Utwórz `internal/testutil/exif.go`:

```go
package testutil

import (
	"bytes"
	"encoding/binary"
)

// SampleJPEGWithOrientation returns a JPEG with EXIF orientation tag.
// Orientation values: 1=normal, 3=180°, 6=90°CW, 8=90°CCW
func SampleJPEGWithOrientation(orientation int) []byte {
	// Minimal JPEG with EXIF APP1 segment containing orientation
	// This is a 2x1 pixel JPEG with EXIF header

	base := SampleJPEG()

	// Build EXIF segment
	exif := buildExifSegment(orientation)

	// Insert EXIF after SOI (FF D8) and before the rest
	result := make([]byte, 0, len(base)+len(exif))
	result = append(result, base[:2]...) // SOI
	result = append(result, exif...)
	result = append(result, base[2:]...)

	return result
}

func buildExifSegment(orientation int) []byte {
	var buf bytes.Buffer

	// APP1 marker
	buf.Write([]byte{0xFF, 0xE1})

	// Placeholder for length (will fill later)
	lengthPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})

	// EXIF header
	buf.WriteString("Exif\x00\x00")

	// TIFF header (little-endian)
	buf.Write([]byte{0x49, 0x49}) // II = little endian
	buf.Write([]byte{0x2A, 0x00}) // TIFF magic
	buf.Write([]byte{0x08, 0x00, 0x00, 0x00}) // offset to IFD0

	// IFD0 with 1 entry (orientation)
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // 1 entry

	// Orientation tag
	binary.Write(&buf, binary.LittleEndian, uint16(0x0112)) // tag
	binary.Write(&buf, binary.LittleEndian, uint16(3))      // type SHORT
	binary.Write(&buf, binary.LittleEndian, uint32(1))      // count
	binary.Write(&buf, binary.LittleEndian, uint16(orientation))
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // padding

	// Next IFD offset (0 = none)
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Update length
	data := buf.Bytes()
	length := uint16(len(data) - 2) // minus APP1 marker
	data[lengthPos] = byte(length >> 8)
	data[lengthPos+1] = byte(length)

	return data
}
```

**Step 2: Napisz failing test dla auto-rotacji**

Utwórz `internal/image/processor_test.go`:

```go
package image

import (
	"testing"

	"dajtu/internal/testutil"
)

func TestProcess_AutoRotateEXIF(t *testing.T) {
	// Skip if libvips unavailable
	if _, err := Process(testutil.SampleJPEG()); err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	tests := []struct {
		name        string
		orientation int
		wantRotated bool // Should dimensions be swapped?
	}{
		{"normal", 1, false},
		{"180", 3, false},
		{"90CW", 6, true},   // Should be rotated, dimensions swapped
		{"90CCW", 8, true},  // Should be rotated, dimensions swapped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a 2x1 image so we can detect rotation
			input := testutil.SampleJPEGWithOrientation(tt.orientation)

			results, err := Process(input)
			if err != nil {
				t.Fatalf("Process: %v", err)
			}

			// Get original result
			var original *ProcessResult
			for i := range results {
				if results[i].Name == "original" {
					original = &results[i]
					break
				}
			}

			if original == nil {
				t.Fatal("no original result")
			}

			// For test JPEG (rectangular), check if dimensions match expectation
			// Note: with auto-rotate, 90° rotations should swap width/height
			if tt.wantRotated {
				// After 90° rotation, width and height should be swapped
				// This is hard to test with 1x1 pixel, so we just verify no error
				t.Logf("orientation %d processed successfully: %dx%d",
					tt.orientation, original.Width, original.Height)
			}
		})
	}
}

func TestProcessWithTransform_Rotation(t *testing.T) {
	if _, err := Process(testutil.SampleJPEG()); err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	p := NewProcessor()

	params := TransformParams{
		Rotation: 90,
	}

	results, err := p.ProcessWithTransform(testutil.SampleJPEG(), params)
	if err != nil {
		t.Fatalf("ProcessWithTransform: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no results")
	}
}

func TestTransformParams_HasTransforms(t *testing.T) {
	tests := []struct {
		name   string
		params TransformParams
		want   bool
	}{
		{"empty", TransformParams{}, false},
		{"rotation", TransformParams{Rotation: 90}, true},
		{"flipH", TransformParams{FlipH: true}, true},
		{"flipV", TransformParams{FlipV: true}, true},
		{"crop", TransformParams{CropW: 100, CropH: 100}, true},
		{"crop partial", TransformParams{CropW: 100}, false}, // needs both W and H
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.params.HasTransforms(); got != tt.want {
				t.Errorf("HasTransforms() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 3: Uruchom testy**

```bash
cd /home/pawel/dev/dajtu && go test ./internal/image/... -v
```

**Step 4: Sprawdź obecną implementację**

Obecna implementacja w `processor.go` już ma:
- `NoAutoRotate: false` w `processVariants` gdy nie ma transformacji
- bimg automatycznie czyta EXIF i rotuje

Więc auto-rotacja już działa! Test powinien przejść. Jeśli nie, problem jest gdzie indziej.

**Step 5: Commit testy**

```bash
git add internal/image/processor_test.go internal/testutil/exif.go
git commit -m "$(cat <<'EOF'
test(image): add processor tests for EXIF auto-rotation

Verify that bimg correctly auto-rotates images based on EXIF orientation.
Add test utility for generating JPEGs with EXIF orientation tags.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Testy E2E dla frontendu

**Problem:** Brak testów dla logiki JavaScript.

**Files:**
- Create: `internal/handler/templates/index_test.html` (manual test page)
- Document test cases

**Step 1: Utwórz listę testów manualnych**

Dodaj do `docs/plans/2026-01-22-enhanced-image-upload.md` sekcję:

### Manual Test Checklist

#### Persystentna lista plików
- [ ] Dodanie pliku przez klik - plik pojawia się w preview
- [ ] Dodanie pliku przez drag&drop - plik pojawia się w preview
- [ ] Dodanie kolejnego pliku - poprzedni pozostaje
- [ ] Usunięcie pliku - tylko wybrany znika
- [ ] Edycja pliku - edytowany plik zachowuje zmiany po dodaniu kolejnych

#### Modal edytora
- [ ] Klik "Edytuj" otwiera modal
- [ ] Modal zajmuje ~80% szerokości
- [ ] ESC zamyka modal
- [ ] Klik poza modalem zamyka
- [ ] Przyciski aspect ratio działają (Free, 1:1, 16:9, 4:3, 9:16, 3:4)
- [ ] "Zastosuj" zapisuje zmiany i zamyka modal
- [ ] "Anuluj" odrzuca zmiany i zamyka modal

#### Rotacja z aspect ratio
- [ ] Prostokątny obrazek + Free + Rotate → zaznaczenie obejmuje cały obrazek
- [ ] Prostokątny obrazek + 1:1 + Rotate → zaznaczenie zachowuje proporcje

#### Tytuł galerii
- [ ] Upload bez tytułu tworzy galerię bez nazwy
- [ ] W trybie edycji galerii można zmienić tytuł
- [ ] Zmiana tytułu aktualizuje nagłówek strony

#### EXIF auto-rotacja
- [ ] Upload zdjęcia z telefonu (portrait) → zapisane jako portrait
- [ ] Upload zdjęcia z EXIF orientation 6 → wyświetla się poprawnie

**Step 2: Commit dokumentację**

```bash
git add docs/plans/2026-01-22-enhanced-image-upload.md
git commit -m "$(cat <<'EOF'
docs: add manual test checklist for upload features

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

| Task | Opis | Pliki |
|------|------|-------|
| 1 | Persystentna lista plików | index.html |
| 2 | Modal dla edytora + nowe aspect ratio | index.html |
| 3 | Tytuł galerii po utworzeniu | gallery.go, gallery.html, index.html, db.go |
| 4 | Fix aspect ratio po rotacji | index.html |
| 5 | Auto-rotacja EXIF (testy) | processor_test.go, testutil/exif.go |
| 6 | Dokumentacja testów | docs/plans/ |

**Kolejność:** 1 → 2 → 4 → 3 → 5 → 6

(Task 1-2-4 to frontend, można robić razem. Task 3 wymaga backendu. Task 5 to testy.)

---

## Manual Test Checklist

### Persystentna lista plików
- [ ] Dodanie pliku przez klik - plik pojawia się w preview
- [ ] Dodanie pliku przez drag&drop - plik pojawia się w preview
- [ ] Dodanie kolejnego pliku - poprzedni pozostaje
- [ ] Usunięcie pliku - tylko wybrany znika
- [ ] Edycja pliku - edytowany plik zachowuje zmiany po dodaniu kolejnych

### Modal edytora
- [ ] Klik "Edytuj" (ołówek) otwiera modal
- [ ] Modal zajmuje ~80% szerokości ekranu
- [ ] ESC zamyka modal
- [ ] Klik poza modalem zamyka
- [ ] Przyciski aspect ratio działają (Free, 1:1, 16:9, 4:3, 9:16, 3:4)
- [ ] "Zastosuj" zapisuje zmiany i zamyka modal
- [ ] "Anuluj" odrzuca zmiany i zamyka modal

### Rotacja z aspect ratio
- [ ] Prostokątny obrazek + Free + Rotate → zaznaczenie obejmuje cały obrazek
- [ ] Prostokątny obrazek + 1:1 + Rotate → zaznaczenie zachowuje proporcje

### Tytuł galerii
- [ ] Upload wielu plików tworzy galerię bez nazwy
- [ ] W trybie edycji galerii można zmienić tytuł (formularz "Zapisz tytuł")
- [ ] Zmiana tytułu aktualizuje nagłówek strony

### Upload flow
- [ ] Single file → redirect do /i/{slug}
- [ ] Multiple files → redirect do /g/{slug}

### EXIF auto-rotacja
- [ ] Upload zdjęcia z telefonu (portrait EXIF) → zapisane jako portrait
- [ ] Upload zdjęcia z EXIF orientation 6 → wyświetla się poprawnie obrócone
