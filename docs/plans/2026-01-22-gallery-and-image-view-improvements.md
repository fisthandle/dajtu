# Gallery and Image View Improvements

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Poprawić widok galerii (brakujące style, usuwanie/edycja zdjęć, dodawanie kolejnych) oraz rozbudować widok pojedynczego obrazka o możliwość tworzenia galerii.

**Architecture:**
- Wyciągnąć wspólne komponenty CSS i JS (modal-editor, cropper) do osobnych plików embedded
- Gallery.html dostaje pełne style dla gridu, lightboxa, panelu edycji
- Image.html dostaje formularz dodawania kolejnego obrazka → tworzy galerię
- Wszystko client-side z autoryzacją przez X-Edit-Token

**Tech Stack:** Go templates, Cropper.js 1.6.2, Vanilla JS, CSS Grid

---

## Task 1: Wyodrębnij wspólne komponenty CSS i JS

**Cel:** Usunąć duplikację kodu edytora między index.html i edit_image.html

**Files:**
- Create: `internal/handler/templates/partials/editor-modal.html`
- Modify: `internal/handler/templates/index.html`
- Modify: `internal/handler/templates/edit_image.html`
- Modify: `internal/handler/handler.go` (jeśli trzeba zmienić ParseFS)

**Step 1: Stwórz partial z edytorem**

```html
{{/* partials/editor-modal.html - Modal edytora obrazków z Cropper.js */}}

<style>
/* ===== EDITOR MODAL STYLES ===== */
.editor-modal {
    display: none;
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(0, 0, 0, 0.95);
    z-index: 9999;
    flex-direction: column;
}

.editor-modal.active {
    display: flex;
}

.editor-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 20px;
    background: #1a1a2e;
    border-bottom: 1px solid #333;
}

.editor-header h3 {
    margin: 0;
    color: #fff;
    font-size: 16px;
}

.editor-close {
    background: none;
    border: none;
    color: #fff;
    font-size: 28px;
    cursor: pointer;
    padding: 0 10px;
    line-height: 1;
}

.editor-close:hover {
    color: #ff6b6b;
}

.editor-body {
    flex: 1;
    display: flex;
    overflow: hidden;
    min-height: 0;
}

.editor-canvas-area {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #111;
    overflow: hidden;
    padding: 20px;
}

.editor-canvas-area img {
    max-width: 100%;
    max-height: 100%;
    display: block;
}

.editor-sidebar {
    width: 200px;
    background: #1a1a2e;
    padding: 15px;
    display: flex;
    flex-direction: column;
    gap: 15px;
    overflow-y: auto;
}

.editor-section h4 {
    color: #aaa;
    font-size: 12px;
    text-transform: uppercase;
    margin: 0 0 10px 0;
    letter-spacing: 1px;
}

.editor-btn-group {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
}

.editor-btn {
    background: #2d2d44;
    border: 1px solid #444;
    color: #fff;
    padding: 8px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
    transition: all 0.2s;
}

.editor-btn:hover {
    background: #3d3d5c;
    border-color: #666;
}

.editor-btn.active {
    background: #4a9eff;
    border-color: #4a9eff;
}

.editor-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.editor-footer {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    padding: 15px 20px;
    background: #1a1a2e;
    border-top: 1px solid #333;
}

.editor-save {
    background: #4a9eff;
    color: #fff;
    border: none;
    padding: 10px 24px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
}

.editor-save:hover {
    background: #3a8eef;
}

.editor-cancel {
    background: #444;
    color: #fff;
    border: none;
    padding: 10px 24px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
}

.editor-cancel:hover {
    background: #555;
}

/* Cropper overrides */
.cropper-container {
    max-width: 100% !important;
    max-height: 100% !important;
}
</style>

<div class="editor-modal" id="editorModal">
    <div class="editor-header">
        <h3>Edytuj obrazek</h3>
        <button class="editor-close" onclick="closeEditor()">&times;</button>
    </div>
    <div class="editor-body">
        <div class="editor-canvas-area">
            <img id="editorImage" src="" alt="Editor">
        </div>
        <div class="editor-sidebar">
            <div class="editor-section">
                <h4>Obrót</h4>
                <div class="editor-btn-group">
                    <button class="editor-btn" onclick="editorRotate(-90)" title="Obróć w lewo">↶ 90°</button>
                    <button class="editor-btn" onclick="editorRotate(90)" title="Obróć w prawo">↷ 90°</button>
                </div>
            </div>
            <div class="editor-section">
                <h4>Odbicie</h4>
                <div class="editor-btn-group">
                    <button class="editor-btn" onclick="editorFlip('h')" title="Odbij poziomo">↔ Poziomo</button>
                    <button class="editor-btn" onclick="editorFlip('v')" title="Odbij pionowo">↕ Pionowo</button>
                </div>
            </div>
            <div class="editor-section">
                <h4>Proporcje</h4>
                <div class="editor-btn-group">
                    <button class="editor-btn aspect-btn" data-ratio="1" onclick="setAspectRatio(1)">1:1</button>
                    <button class="editor-btn aspect-btn" data-ratio="1.778" onclick="setAspectRatio(16/9)">16:9</button>
                    <button class="editor-btn aspect-btn" data-ratio="1.333" onclick="setAspectRatio(4/3)">4:3</button>
                    <button class="editor-btn aspect-btn active" data-ratio="0" onclick="setAspectRatio(NaN)">Wolne</button>
                </div>
            </div>
            <div class="editor-section">
                <h4>Historia</h4>
                <div class="editor-btn-group">
                    <button class="editor-btn" id="undoBtn" onclick="editorUndo()" disabled>↩ Cofnij</button>
                    <button class="editor-btn" id="redoBtn" onclick="editorRedo()" disabled>↪ Ponów</button>
                </div>
            </div>
        </div>
    </div>
    <div class="editor-footer">
        <button class="editor-cancel" onclick="closeEditor()">Anuluj</button>
        <button class="editor-save" onclick="saveEditorChanges()">Zapisz</button>
    </div>
</div>

<script src="https://cdnjs.cloudflare.com/ajax/libs/cropperjs/1.6.2/cropper.min.js"></script>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/cropperjs/1.6.2/cropper.min.css">

<script>
/* ===== EDITOR MODAL LOGIC ===== */
let cropper = null;
let editorHistory = [];
let editorHistoryIndex = -1;
let currentEditingFile = null;
let onEditorSave = null;

function openEditor(imageSource, callback) {
    const modal = document.getElementById('editorModal');
    const img = document.getElementById('editorImage');

    onEditorSave = callback;
    editorHistory = [];
    editorHistoryIndex = -1;

    if (typeof imageSource === 'string') {
        img.src = imageSource;
    } else if (imageSource instanceof File) {
        currentEditingFile = imageSource;
        img.src = URL.createObjectURL(imageSource);
    } else if (imageSource instanceof Blob) {
        img.src = URL.createObjectURL(imageSource);
    }

    modal.classList.add('active');
    document.body.style.overflow = 'hidden';

    img.onload = function() {
        if (cropper) cropper.destroy();
        cropper = new Cropper(img, {
            viewMode: 1,
            dragMode: 'move',
            autoCropArea: 1,
            responsive: true,
            restore: false,
            guides: true,
            center: true,
            highlight: true,
            cropBoxMovable: true,
            cropBoxResizable: true,
            toggleDragModeOnDblclick: true,
            ready: function() {
                saveEditorState();
            }
        });
    };
}

function closeEditor() {
    const modal = document.getElementById('editorModal');
    modal.classList.remove('active');
    document.body.style.overflow = '';

    if (cropper) {
        cropper.destroy();
        cropper = null;
    }

    const img = document.getElementById('editorImage');
    if (img.src.startsWith('blob:')) {
        URL.revokeObjectURL(img.src);
    }
    img.src = '';

    currentEditingFile = null;
    onEditorSave = null;
}

function editorRotate(degrees) {
    if (!cropper) return;
    cropper.rotate(degrees);
    saveEditorState();
}

function editorFlip(direction) {
    if (!cropper) return;
    const data = cropper.getData();
    if (direction === 'h') {
        cropper.scaleX(data.scaleX === -1 ? 1 : -1);
    } else {
        cropper.scaleY(data.scaleY === -1 ? 1 : -1);
    }
    saveEditorState();
}

function setAspectRatio(ratio) {
    if (!cropper) return;
    cropper.setAspectRatio(ratio);

    document.querySelectorAll('.aspect-btn').forEach(btn => {
        btn.classList.remove('active');
        const btnRatio = parseFloat(btn.dataset.ratio);
        if ((isNaN(ratio) && btnRatio === 0) || Math.abs(btnRatio - ratio) < 0.01) {
            btn.classList.add('active');
        }
    });
    saveEditorState();
}

function saveEditorState() {
    if (!cropper) return;

    editorHistory = editorHistory.slice(0, editorHistoryIndex + 1);
    editorHistory.push(cropper.getData());
    editorHistoryIndex = editorHistory.length - 1;

    updateHistoryButtons();
}

function updateHistoryButtons() {
    document.getElementById('undoBtn').disabled = editorHistoryIndex <= 0;
    document.getElementById('redoBtn').disabled = editorHistoryIndex >= editorHistory.length - 1;
}

function editorUndo() {
    if (editorHistoryIndex > 0) {
        editorHistoryIndex--;
        cropper.setData(editorHistory[editorHistoryIndex]);
        updateHistoryButtons();
    }
}

function editorRedo() {
    if (editorHistoryIndex < editorHistory.length - 1) {
        editorHistoryIndex++;
        cropper.setData(editorHistory[editorHistoryIndex]);
        updateHistoryButtons();
    }
}

function saveEditorChanges() {
    if (!cropper || !onEditorSave) return;

    const canvas = cropper.getCroppedCanvas({
        maxWidth: 4096,
        maxHeight: 4096,
        imageSmoothingEnabled: true,
        imageSmoothingQuality: 'high'
    });

    canvas.toBlob(function(blob) {
        onEditorSave(blob, cropper.getData());
        closeEditor();
    }, 'image/jpeg', 0.92);
}

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    if (!document.getElementById('editorModal').classList.contains('active')) return;

    if (e.key === 'Escape') {
        closeEditor();
    } else if (e.ctrlKey && e.key === 'z') {
        e.preventDefault();
        editorUndo();
    } else if (e.ctrlKey && e.key === 'y') {
        e.preventDefault();
        editorRedo();
    }
});
</script>
```

**Step 2: Zmodyfikuj index.html - użyj partial**

W index.html usuń cały kod edytora (style + modal + JS) i zamień na:
```html
{{template "partials/editor-modal.html"}}
```

Dostosuj wywołanie edytora w index.html:
```javascript
// Zamiast bezpośredniej manipulacji, użyj openEditor():
openEditor(file, function(blob, data) {
    // blob to wynikowy obrazek
    // data to dane croppera (rotacja, crop, etc.)
    editedFiles.set(file.name, blob);
    updatePreview(file.name, blob);
});
```

**Step 3: Zmodyfikuj edit_image.html - użyj partial**

Usuń zduplikowany kod edytora i użyj partial. Dostosuj wywołanie:
```html
{{template "partials/editor-modal.html"}}

<script>
// Otwórz edytor z URL obrazka
openEditor('{{.BaseURL}}/i/{{.Image.Slug}}/original', function(blob, data) {
    // Wyślij na serwer
    const formData = new FormData();
    formData.append('file', blob, 'edited.jpg');
    formData.append('rotation', data.rotate || 0);

    fetch('{{.BaseURL}}/i/{{.Image.Slug}}/edit', {
        method: 'POST',
        headers: { 'X-Edit-Token': '{{.EditToken}}' },
        body: formData
    }).then(r => r.json()).then(result => {
        window.location.reload();
    });
});
</script>
```

**Step 4: Zweryfikuj że ParseFS ładuje partials**

Sprawdź `internal/handler/handler.go` czy templates są parsowane z subdirectories:
```go
// Powinno być:
templates, err := template.ParseFS(templatesFS, "templates/*.html", "templates/partials/*.html")
```

**Step 5: Zbuduj i przetestuj**

Run: `go build -o dajtu ./cmd/dajtu`
Expected: Build successful

**Step 6: Commit**

```bash
git add internal/handler/templates/partials/editor-modal.html internal/handler/templates/index.html internal/handler/templates/edit_image.html internal/handler/handler.go
git commit -m "refactor: extract editor modal to shared partial"
```

---

## Task 2: Dodaj brakujące style do gallery.html

**Cel:** Gallery.html ma HTML ale brak 90% CSS - dodać style dla gridu, lightboxa, edit-panelu, pagination

**Files:**
- Modify: `internal/handler/templates/gallery.html`

**Step 1: Dodaj kompletne style CSS**

Dodaj w sekcji `<style>` w gallery.html:

```css
/* ===== RESET & BASE ===== */
* {
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0a0a0f;
    color: #e0e0e0;
    margin: 0;
    padding: 20px;
    min-height: 100vh;
}

/* ===== GALLERY GRID ===== */
.gallery {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 16px;
    padding: 20px 0;
}

.gallery-item {
    position: relative;
    aspect-ratio: 1;
    background: #1a1a2e;
    border-radius: 8px;
    overflow: hidden;
    cursor: pointer;
    transition: transform 0.2s, box-shadow 0.2s;
}

.gallery-item:hover {
    transform: translateY(-4px);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
}

.gallery-item img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    transition: opacity 0.3s;
}

.gallery-item img[data-src] {
    opacity: 0;
}

.gallery-item img.loaded {
    opacity: 1;
}

/* ===== DELETE & EDIT BUTTONS ===== */
.gallery-item .item-actions {
    position: absolute;
    top: 8px;
    right: 8px;
    display: flex;
    gap: 6px;
    opacity: 0;
    transition: opacity 0.2s;
}

.gallery-item:hover .item-actions {
    opacity: 1;
}

.gallery-item .delete-btn,
.gallery-item .edit-btn {
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 50%;
    cursor: pointer;
    font-size: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
}

.gallery-item .delete-btn {
    background: rgba(255, 100, 100, 0.9);
    color: #fff;
}

.gallery-item .delete-btn:hover {
    background: #ff4444;
    transform: scale(1.1);
}

.gallery-item .edit-btn {
    background: rgba(74, 158, 255, 0.9);
    color: #fff;
}

.gallery-item .edit-btn:hover {
    background: #4a9eff;
    transform: scale(1.1);
}

/* ===== EDIT PANEL ===== */
.edit-panel {
    background: #1a1a2e;
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 20px;
}

.edit-panel h3 {
    margin: 0 0 15px 0;
    color: #fff;
    font-size: 16px;
}

.edit-panel .links-table {
    width: 100%;
    border-collapse: collapse;
}

.edit-panel .links-table td {
    padding: 8px 0;
    border-bottom: 1px solid #2d2d44;
}

.edit-panel .links-table td:first-child {
    color: #888;
    width: 120px;
}

.edit-panel .links-table input[type="text"] {
    width: 100%;
    background: #0d0d14;
    border: 1px solid #333;
    color: #fff;
    padding: 8px 12px;
    border-radius: 4px;
    font-family: monospace;
    font-size: 13px;
}

.edit-panel .copy-btn {
    background: #2d2d44;
    border: 1px solid #444;
    color: #fff;
    padding: 8px 16px;
    border-radius: 4px;
    cursor: pointer;
    margin-left: 8px;
    transition: all 0.2s;
}

.edit-panel .copy-btn:hover {
    background: #3d3d5c;
}

/* ===== TITLE EDITOR ===== */
.title-editor {
    display: flex;
    gap: 10px;
    margin-bottom: 20px;
}

.title-editor input {
    flex: 1;
    background: #0d0d14;
    border: 1px solid #333;
    color: #fff;
    padding: 10px 14px;
    border-radius: 6px;
    font-size: 16px;
}

.title-editor button {
    background: #4a9eff;
    border: none;
    color: #fff;
    padding: 10px 20px;
    border-radius: 6px;
    cursor: pointer;
    font-weight: 500;
}

.title-editor button:hover {
    background: #3a8eef;
}

/* ===== ADD IMAGE FORM ===== */
.add-image-section {
    background: #1a1a2e;
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 20px;
}

.add-image-section h3 {
    margin: 0 0 15px 0;
    color: #fff;
    font-size: 16px;
}

.add-image-dropzone {
    border: 2px dashed #444;
    border-radius: 8px;
    padding: 40px;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s;
}

.add-image-dropzone:hover,
.add-image-dropzone.dragover {
    border-color: #4a9eff;
    background: rgba(74, 158, 255, 0.1);
}

.add-image-dropzone input {
    display: none;
}

.add-image-dropzone p {
    margin: 0;
    color: #888;
}

.add-image-dropzone .icon {
    font-size: 48px;
    margin-bottom: 10px;
    color: #555;
}

/* ===== LIGHTBOX ===== */
.lightbox {
    display: none;
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(0, 0, 0, 0.95);
    z-index: 9000;
    align-items: center;
    justify-content: center;
}

.lightbox.active {
    display: flex;
}

.lightbox-content {
    position: relative;
    max-width: 90vw;
    max-height: 90vh;
}

.lightbox-content img {
    max-width: 90vw;
    max-height: 90vh;
    object-fit: contain;
}

.lightbox-close {
    position: absolute;
    top: 20px;
    right: 20px;
    background: none;
    border: none;
    color: #fff;
    font-size: 36px;
    cursor: pointer;
    z-index: 9001;
}

.lightbox-nav {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    background: rgba(255, 255, 255, 0.1);
    border: none;
    color: #fff;
    font-size: 48px;
    padding: 20px;
    cursor: pointer;
    transition: background 0.2s;
}

.lightbox-nav:hover {
    background: rgba(255, 255, 255, 0.2);
}

.lightbox-prev {
    left: 20px;
}

.lightbox-next {
    right: 20px;
}

/* ===== PAGINATION ===== */
.pagination {
    display: flex;
    justify-content: center;
    gap: 8px;
    padding: 20px 0;
}

.pagination a,
.pagination span {
    padding: 10px 16px;
    border-radius: 6px;
    text-decoration: none;
    transition: all 0.2s;
}

.pagination a {
    background: #1a1a2e;
    color: #fff;
}

.pagination a:hover {
    background: #2d2d44;
}

.pagination span.current {
    background: #4a9eff;
    color: #fff;
}

.pagination span.disabled {
    background: #1a1a2e;
    color: #555;
}

/* ===== HEADER ===== */
.gallery-header {
    display: flex;
    align-items: center;
    gap: 20px;
    margin-bottom: 20px;
}

.gallery-header .logo {
    width: 48px;
    height: 48px;
}

.gallery-header h1 {
    margin: 0;
    font-size: 24px;
    color: #fff;
}

.gallery-header .image-count {
    color: #888;
    font-size: 14px;
}

/* ===== EMPTY STATE ===== */
.empty-gallery {
    text-align: center;
    padding: 60px 20px;
    color: #888;
}

.empty-gallery .icon {
    font-size: 64px;
    margin-bottom: 20px;
    color: #444;
}

.empty-gallery p {
    font-size: 18px;
    margin: 0;
}

/* ===== LOADING ===== */
.loading-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.8);
    display: none;
    align-items: center;
    justify-content: center;
    z-index: 10000;
}

.loading-overlay.active {
    display: flex;
}

.loading-spinner {
    width: 50px;
    height: 50px;
    border: 3px solid #333;
    border-top-color: #4a9eff;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}

/* ===== RESPONSIVE ===== */
@media (max-width: 768px) {
    .gallery {
        grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
        gap: 10px;
    }

    .edit-panel {
        padding: 15px;
    }

    .gallery-item .item-actions {
        opacity: 1;
    }
}
```

**Step 2: Zbuduj i przetestuj**

Run: `go build -o dajtu ./cmd/dajtu`
Expected: Build successful

**Step 3: Commit**

```bash
git add internal/handler/templates/gallery.html
git commit -m "style: add complete CSS for gallery grid, lightbox, and edit panel"
```

---

## Task 3: Dodaj przyciski usuwania i edycji do zdjęć w galerii

**Cel:** Każde zdjęcie ma przycisk X (usuń) i ✎ (edytuj), widoczne tylko w edit mode

**Files:**
- Modify: `internal/handler/templates/gallery.html`

**Step 1: Zmodyfikuj HTML grid item**

Zamień strukturę `.gallery-item` w gallery.html:

```html
{{range .Images}}
<div class="gallery-item" data-slug="{{.Slug}}">
    {{if $.EditMode}}
    <div class="item-actions">
        <button class="edit-btn" onclick="editImage('{{.Slug}}')" title="Edytuj">✎</button>
        <button class="delete-btn" onclick="deleteImage('{{.Slug}}')" title="Usuń">&times;</button>
    </div>
    {{end}}
    <a href="javascript:void(0)" onclick="openLightbox('{{.Slug}}')" data-full="{{$.BaseURL}}/i/{{.Slug}}/1920">
        <img data-src="{{$.BaseURL}}/i/{{.Slug}}/200" alt="" class="lazy">
    </a>
</div>
{{end}}
```

**Step 2: Dodaj JavaScript dla usuwania**

```javascript
const editToken = '{{.EditToken}}';
const gallerySlug = '{{.Slug}}';
const baseURL = '{{.BaseURL}}';

async function deleteImage(imageSlug) {
    if (!confirm('Usunąć ten obrazek?')) return;

    showLoading();
    try {
        const response = await fetch(`${baseURL}/gallery/${gallerySlug}/${imageSlug}`, {
            method: 'DELETE',
            headers: { 'X-Edit-Token': editToken }
        });

        if (response.ok) {
            document.querySelector(`.gallery-item[data-slug="${imageSlug}"]`).remove();

            // Sprawdź czy galeria pusta
            if (document.querySelectorAll('.gallery-item').length === 0) {
                document.getElementById('gallery').innerHTML = `
                    <div class="empty-gallery">
                        <div class="icon">📭</div>
                        <p>Galeria jest pusta. Dodaj zdjęcia poniżej.</p>
                    </div>
                `;
            }
        } else {
            alert('Błąd podczas usuwania');
        }
    } catch (e) {
        alert('Błąd połączenia');
    }
    hideLoading();
}
```

**Step 3: Dodaj JavaScript dla edycji (wymaga Task 1)**

```javascript
function editImage(imageSlug) {
    const imageUrl = `${baseURL}/i/${imageSlug}/original`;

    openEditor(imageUrl, async function(blob, data) {
        showLoading();
        try {
            const formData = new FormData();
            formData.append('file', blob, 'edited.jpg');

            const response = await fetch(`${baseURL}/i/${imageSlug}/edit`, {
                method: 'POST',
                headers: { 'X-Edit-Token': editToken },
                body: formData
            });

            if (response.ok) {
                // Odśwież miniaturkę
                const img = document.querySelector(`.gallery-item[data-slug="${imageSlug}"] img`);
                img.src = `${baseURL}/i/${imageSlug}/200?t=${Date.now()}`;
            } else {
                alert('Błąd podczas zapisywania');
            }
        } catch (e) {
            alert('Błąd połączenia');
        }
        hideLoading();
    });
}
```

**Step 4: Dodaj loading overlay do HTML**

```html
<div class="loading-overlay" id="loadingOverlay">
    <div class="loading-spinner"></div>
</div>

<script>
function showLoading() {
    document.getElementById('loadingOverlay').classList.add('active');
}
function hideLoading() {
    document.getElementById('loadingOverlay').classList.remove('active');
}
</script>
```

**Step 5: Commit**

```bash
git add internal/handler/templates/gallery.html
git commit -m "feat: add delete and edit buttons for gallery images"
```

---

## Task 4: Dodaj formularz dodawania obrazków do galerii

**Cel:** W widoku galerii (edit mode) formularz drag&drop do dodawania kolejnych zdjęć

**Files:**
- Modify: `internal/handler/templates/gallery.html`

**Step 1: Dodaj HTML formularza**

Po edit-panel, przed galerią:

```html
{{if .EditMode}}
<div class="add-image-section">
    <h3>Dodaj zdjęcia</h3>
    <div class="add-image-dropzone" id="addDropzone">
        <input type="file" id="addFileInput" multiple accept="image/*">
        <div class="icon">📤</div>
        <p>Przeciągnij zdjęcia lub kliknij aby wybrać</p>
    </div>
    <div class="upload-progress" id="uploadProgress" style="display: none;">
        <div class="progress-bar">
            <div class="progress-fill" id="progressFill"></div>
        </div>
        <p id="progressText">Wysyłanie...</p>
    </div>
</div>
{{end}}
```

**Step 2: Dodaj style progress bara**

```css
.upload-progress {
    margin-top: 15px;
}

.progress-bar {
    height: 8px;
    background: #2d2d44;
    border-radius: 4px;
    overflow: hidden;
}

.progress-fill {
    height: 100%;
    background: #4a9eff;
    width: 0%;
    transition: width 0.3s;
}

.upload-progress p {
    margin: 10px 0 0 0;
    color: #888;
    font-size: 14px;
}
```

**Step 3: Dodaj JavaScript upload**

```javascript
const dropzone = document.getElementById('addDropzone');
const fileInput = document.getElementById('addFileInput');

dropzone.addEventListener('click', () => fileInput.click());

dropzone.addEventListener('dragover', (e) => {
    e.preventDefault();
    dropzone.classList.add('dragover');
});

dropzone.addEventListener('dragleave', () => {
    dropzone.classList.remove('dragover');
});

dropzone.addEventListener('drop', (e) => {
    e.preventDefault();
    dropzone.classList.remove('dragover');
    handleFiles(e.dataTransfer.files);
});

fileInput.addEventListener('change', () => {
    handleFiles(fileInput.files);
});

async function handleFiles(files) {
    if (files.length === 0) return;

    const progress = document.getElementById('uploadProgress');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');

    progress.style.display = 'block';
    progressFill.style.width = '0%';

    const formData = new FormData();
    for (const file of files) {
        formData.append('files', file);
    }

    try {
        const xhr = new XMLHttpRequest();

        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
                const percent = (e.loaded / e.total) * 100;
                progressFill.style.width = percent + '%';
                progressText.textContent = `Wysyłanie... ${Math.round(percent)}%`;
            }
        };

        xhr.onload = function() {
            if (xhr.status === 200) {
                const result = JSON.parse(xhr.responseText);
                addImagesToGallery(result.images);
                progress.style.display = 'none';
                fileInput.value = '';
            } else {
                alert('Błąd podczas wysyłania');
                progress.style.display = 'none';
            }
        };

        xhr.onerror = () => {
            alert('Błąd połączenia');
            progress.style.display = 'none';
        };

        xhr.open('POST', `${baseURL}/gallery/${gallerySlug}/add`);
        xhr.setRequestHeader('X-Edit-Token', editToken);
        xhr.send(formData);

    } catch (e) {
        alert('Błąd: ' + e.message);
        progress.style.display = 'none';
    }
}

function addImagesToGallery(images) {
    const gallery = document.getElementById('gallery');

    // Usuń empty state jeśli był
    const emptyState = gallery.querySelector('.empty-gallery');
    if (emptyState) emptyState.remove();

    for (const img of images) {
        const item = document.createElement('div');
        item.className = 'gallery-item';
        item.dataset.slug = img.slug;
        item.innerHTML = `
            <div class="item-actions">
                <button class="edit-btn" onclick="editImage('${img.slug}')" title="Edytuj">✎</button>
                <button class="delete-btn" onclick="deleteImage('${img.slug}')" title="Usuń">&times;</button>
            </div>
            <a href="javascript:void(0)" onclick="openLightbox('${img.slug}')" data-full="${baseURL}/i/${img.slug}/1920">
                <img src="${baseURL}/i/${img.slug}/200" alt="" class="loaded">
            </a>
        `;
        gallery.appendChild(item);
    }
}
```

**Step 4: Commit**

```bash
git add internal/handler/templates/gallery.html
git commit -m "feat: add drag-drop form to add images to gallery"
```

---

## Task 5: Rozbuduj widok pojedynczego obrazka

**Cel:** Zamiast samego obrazka, pokazuj linki i opcję stworzenia galerii

**Files:**
- Modify: `internal/handler/templates/image.html`
- Modify: `internal/handler/upload.go` (redirect)

**Step 1: Sprawdź aktualny template image.html**

Przeczytaj `internal/handler/templates/image.html` i zidentyfikuj co wymaga zmian.

**Step 2: Dodaj kompletne style do image.html**

```css
* {
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0a0a0f;
    color: #e0e0e0;
    margin: 0;
    padding: 20px;
    min-height: 100vh;
}

.container {
    max-width: 1200px;
    margin: 0 auto;
}

.header {
    display: flex;
    align-items: center;
    gap: 15px;
    margin-bottom: 20px;
}

.header .logo {
    width: 40px;
    height: 40px;
}

.header h1 {
    margin: 0;
    font-size: 20px;
    color: #fff;
}

.image-preview {
    background: #1a1a2e;
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 20px;
    text-align: center;
}

.image-preview img {
    max-width: 100%;
    max-height: 70vh;
    border-radius: 8px;
}

.links-section {
    background: #1a1a2e;
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 20px;
}

.links-section h3 {
    margin: 0 0 15px 0;
    color: #fff;
    font-size: 16px;
}

.link-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
}

.link-row:last-child {
    margin-bottom: 0;
}

.link-row label {
    width: 100px;
    color: #888;
    font-size: 14px;
}

.link-row input {
    flex: 1;
    background: #0d0d14;
    border: 1px solid #333;
    color: #fff;
    padding: 10px 12px;
    border-radius: 6px;
    font-family: monospace;
    font-size: 13px;
}

.link-row button {
    background: #2d2d44;
    border: 1px solid #444;
    color: #fff;
    padding: 10px 16px;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
}

.link-row button:hover {
    background: #3d3d5c;
}

.actions-section {
    background: #1a1a2e;
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 20px;
}

.actions-section h3 {
    margin: 0 0 15px 0;
    color: #fff;
    font-size: 16px;
}

.action-buttons {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
}

.btn {
    padding: 12px 24px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    border: none;
    transition: all 0.2s;
}

.btn-primary {
    background: #4a9eff;
    color: #fff;
}

.btn-primary:hover {
    background: #3a8eef;
}

.btn-secondary {
    background: #2d2d44;
    color: #fff;
    border: 1px solid #444;
}

.btn-secondary:hover {
    background: #3d3d5c;
}

.btn-danger {
    background: #ff4444;
    color: #fff;
}

.btn-danger:hover {
    background: #ee3333;
}

/* Create gallery section */
.create-gallery-section {
    background: #1a1a2e;
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 20px;
}

.create-gallery-section h3 {
    margin: 0 0 15px 0;
    color: #fff;
    font-size: 16px;
}

.create-gallery-dropzone {
    border: 2px dashed #444;
    border-radius: 8px;
    padding: 40px;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s;
}

.create-gallery-dropzone:hover,
.create-gallery-dropzone.dragover {
    border-color: #4a9eff;
    background: rgba(74, 158, 255, 0.1);
}

.create-gallery-dropzone input {
    display: none;
}

.create-gallery-dropzone .icon {
    font-size: 48px;
    margin-bottom: 10px;
    color: #555;
}

.create-gallery-dropzone p {
    margin: 0;
    color: #888;
}

/* Loading */
.loading-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.8);
    display: none;
    align-items: center;
    justify-content: center;
    z-index: 10000;
}

.loading-overlay.active {
    display: flex;
}

.loading-spinner {
    width: 50px;
    height: 50px;
    border: 3px solid #333;
    border-top-color: #4a9eff;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}
```

**Step 3: Zaktualizuj HTML image.html**

```html
<!DOCTYPE html>
<html lang="pl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Image.OriginalName}} - Dajtu</title>
    <style>
    /* ... style z Step 2 ... */
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <a href="{{.BaseURL}}/">
                <img src="{{.BaseURL}}/static/logo.svg" alt="Dajtu" class="logo">
            </a>
            <h1>{{.Image.OriginalName}}</h1>
        </div>

        <div class="image-preview">
            <img src="{{.BaseURL}}/i/{{.Image.Slug}}/1920" alt="{{.Image.OriginalName}}">
        </div>

        <div class="links-section">
            <h3>Linki do obrazka</h3>
            <div class="link-row">
                <label>Bezpośredni:</label>
                <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/original" id="linkOriginal">
                <button onclick="copyLink('linkOriginal')">Kopiuj</button>
            </div>
            <div class="link-row">
                <label>1920px:</label>
                <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/1920" id="link1920">
                <button onclick="copyLink('link1920')">Kopiuj</button>
            </div>
            <div class="link-row">
                <label>800px:</label>
                <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/800" id="link800">
                <button onclick="copyLink('link800')">Kopiuj</button>
            </div>
            <div class="link-row">
                <label>Miniatura:</label>
                <input type="text" readonly value="{{.BaseURL}}/i/{{.Image.Slug}}/thumb" id="linkThumb">
                <button onclick="copyLink('linkThumb')">Kopiuj</button>
            </div>
            <div class="link-row">
                <label>BBCode:</label>
                <input type="text" readonly value="[img]{{.BaseURL}}/i/{{.Image.Slug}}/1920[/img]" id="linkBBCode">
                <button onclick="copyLink('linkBBCode')">Kopiuj</button>
            </div>
            <div class="link-row">
                <label>HTML:</label>
                <input type="text" readonly value='<img src="{{.BaseURL}}/i/{{.Image.Slug}}/1920" alt="">' id="linkHTML">
                <button onclick="copyLink('linkHTML')">Kopiuj</button>
            </div>
        </div>

        {{if .EditMode}}
        <div class="actions-section">
            <h3>Zarządzanie</h3>
            <div class="action-buttons">
                <button class="btn btn-primary" onclick="editImage()">✎ Edytuj obrazek</button>
                <button class="btn btn-danger" onclick="deleteImage()">🗑 Usuń</button>
            </div>
        </div>

        <div class="create-gallery-section">
            <h3>Utwórz galerię</h3>
            <p style="color: #888; margin-bottom: 15px;">Dodaj więcej zdjęć aby utworzyć galerię z tego obrazka</p>
            <div class="create-gallery-dropzone" id="createGalleryDropzone">
                <input type="file" id="galleryFileInput" multiple accept="image/*">
                <div class="icon">📸</div>
                <p>Przeciągnij zdjęcia lub kliknij aby wybrać</p>
            </div>
        </div>
        {{end}}
    </div>

    <div class="loading-overlay" id="loadingOverlay">
        <div class="loading-spinner"></div>
    </div>

    {{if .EditMode}}
    {{template "partials/editor-modal.html"}}
    {{end}}

    <script>
    const baseURL = '{{.BaseURL}}';
    const imageSlug = '{{.Image.Slug}}';
    const editToken = '{{.EditToken}}';

    function copyLink(inputId) {
        const input = document.getElementById(inputId);
        input.select();
        document.execCommand('copy');

        const btn = input.nextElementSibling;
        const originalText = btn.textContent;
        btn.textContent = '✓';
        setTimeout(() => btn.textContent = originalText, 1000);
    }

    function showLoading() {
        document.getElementById('loadingOverlay').classList.add('active');
    }

    function hideLoading() {
        document.getElementById('loadingOverlay').classList.remove('active');
    }

    {{if .EditMode}}
    function editImage() {
        openEditor(`${baseURL}/i/${imageSlug}/original`, async function(blob, data) {
            showLoading();
            try {
                const formData = new FormData();
                formData.append('file', blob, 'edited.jpg');

                const response = await fetch(`${baseURL}/i/${imageSlug}/edit`, {
                    method: 'POST',
                    headers: { 'X-Edit-Token': editToken },
                    body: formData
                });

                if (response.ok) {
                    window.location.reload();
                } else {
                    alert('Błąd podczas zapisywania');
                }
            } catch (e) {
                alert('Błąd połączenia');
            }
            hideLoading();
        });
    }

    async function deleteImage() {
        if (!confirm('Czy na pewno usunąć ten obrazek?')) return;

        showLoading();
        try {
            const response = await fetch(`${baseURL}/i/${imageSlug}`, {
                method: 'DELETE',
                headers: { 'X-Edit-Token': editToken }
            });

            if (response.ok) {
                window.location.href = baseURL + '/';
            } else {
                alert('Błąd podczas usuwania');
            }
        } catch (e) {
            alert('Błąd połączenia');
        }
        hideLoading();
    }

    // Create gallery from this image
    const dropzone = document.getElementById('createGalleryDropzone');
    const fileInput = document.getElementById('galleryFileInput');

    dropzone.addEventListener('click', () => fileInput.click());

    dropzone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropzone.classList.add('dragover');
    });

    dropzone.addEventListener('dragleave', () => {
        dropzone.classList.remove('dragover');
    });

    dropzone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropzone.classList.remove('dragover');
        createGalleryWithFiles(e.dataTransfer.files);
    });

    fileInput.addEventListener('change', () => {
        createGalleryWithFiles(fileInput.files);
    });

    async function createGalleryWithFiles(files) {
        if (files.length === 0) return;

        showLoading();
        try {
            const formData = new FormData();
            formData.append('existing_image', imageSlug);
            for (const file of files) {
                formData.append('files', file);
            }

            const response = await fetch(`${baseURL}/gallery`, {
                method: 'POST',
                headers: { 'X-Edit-Token': editToken },
                body: formData
            });

            if (response.ok) {
                const result = await response.json();
                window.location.href = `${baseURL}/g/${result.slug}?edit=${result.edit_token}`;
            } else {
                alert('Błąd podczas tworzenia galerii');
            }
        } catch (e) {
            alert('Błąd połączenia');
        }
        hideLoading();
    }
    {{end}}
    </script>
</body>
</html>
```

**Step 4: Commit**

```bash
git add internal/handler/templates/image.html
git commit -m "feat: enhance single image view with links and create gallery option"
```

---

## Task 6: Endpoint do tworzenia galerii z istniejącego obrazka

**Cel:** POST /gallery z parametrem existing_image dodaje istniejący obrazek do nowej galerii

**Files:**
- Modify: `internal/handler/gallery.go`

**Step 1: Zmodyfikuj Create handler**

W `Create` dodaj obsługę parametru `existing_image`:

```go
func (h *GalleryHandler) Create(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...

    // Check for existing image to include
    existingImageSlug := r.FormValue("existing_image")
    if existingImageSlug != "" {
        // Verify edit token matches existing image
        existingImage, err := h.storage.GetImageBySlug(existingImageSlug)
        if err != nil {
            http.Error(w, "Image not found", http.StatusNotFound)
            return
        }

        editToken := r.Header.Get("X-Edit-Token")
        if editToken == "" {
            editToken = r.FormValue("edit_token")
        }

        if existingImage.EditToken != editToken {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Move existing image to new gallery
        gallery, err := h.storage.CreateGallery(userID, title)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // Add existing image to gallery
        err = h.storage.AddImageToGallery(gallery.ID, existingImage.ID)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // Process uploaded files...
        // ... rest of existing code ...
    }
}
```

**Step 2: Dodaj metodę storage AddImageToGallery**

W `internal/storage/storage.go`:

```go
func (s *Storage) AddImageToGallery(galleryID, imageID int64) error {
    _, err := s.db.Exec(`UPDATE images SET gallery_id = ? WHERE id = ?`, galleryID, imageID)
    return err
}
```

**Step 3: Commit**

```bash
git add internal/handler/gallery.go internal/storage/storage.go
git commit -m "feat: support creating gallery from existing image"
```

---

## Task 7: Zmień redirect po pojedynczym uploadzie

**Cel:** Pojedynczy upload kieruje do widoku obrazka /i/{slug}?edit={token}, nie do surowego pliku

**Files:**
- Modify: `internal/handler/upload.go`

**Step 1: Znajdź redirect w ServeHTTP**

Zmień:
```go
// Było: redirect do /i/{slug}/original
http.Redirect(w, r, fmt.Sprintf("/i/%s/original", img.Slug), http.StatusSeeOther)

// Powinno być: redirect do widoku z edit tokenem
http.Redirect(w, r, fmt.Sprintf("/i/%s?edit=%s", img.Slug, img.EditToken), http.StatusSeeOther)
```

**Step 2: Dla JSON response też zwróć view_url**

```go
response := map[string]interface{}{
    "slug":       img.Slug,
    "url":        baseURL + "/i/" + img.Slug + "/original",
    "view_url":   baseURL + "/i/" + img.Slug,
    "edit_token": img.EditToken,
    "sizes": map[string]string{
        "original": baseURL + "/i/" + img.Slug + "/original",
        "1920":     baseURL + "/i/" + img.Slug + "/1920",
        "800":      baseURL + "/i/" + img.Slug + "/800",
        "thumb":    baseURL + "/i/" + img.Slug + "/thumb",
    },
}
```

**Step 3: Commit**

```bash
git add internal/handler/upload.go
git commit -m "fix: redirect single upload to view page with edit token"
```

---

## Task 8: Integracja i testy

**Cel:** Wszystko działa razem, testy przechodzą

**Files:**
- Run tests
- Manual testing

**Step 1: Build**

Run: `go build -o dajtu ./cmd/dajtu`
Expected: Build successful

**Step 2: Run tests**

Run: `go test ./...`
Expected: All tests pass

**Step 3: Manual testing checklist**

1. [ ] Upload pojedynczego obrazka → widok z linkami
2. [ ] Edycja obrazka z widoku (cropper)
3. [ ] Usunięcie obrazka z widoku
4. [ ] Tworzenie galerii z pojedynczego obrazka
5. [ ] Widok galerii ze stylami
6. [ ] Usuwanie zdjęć z galerii
7. [ ] Edycja zdjęć w galerii
8. [ ] Dodawanie zdjęć do galerii
9. [ ] Lightbox w galerii
10. [ ] Paginacja w galerii

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete gallery and image view improvements"
```

---

## Summary

| Task | Opis | Pliki |
|------|------|-------|
| 1 | Wyodrębnij edytor do partial | partials/editor-modal.html, index.html, edit_image.html |
| 2 | Style CSS dla galerii | gallery.html |
| 3 | Przyciski usuwania/edycji | gallery.html |
| 4 | Formularz dodawania | gallery.html |
| 5 | Widok pojedynczego obrazka | image.html |
| 6 | Endpoint tworzenia galerii | gallery.go, storage.go |
| 7 | Redirect po uploadzie | upload.go |
| 8 | Integracja i testy | - |

**Kluczowe decyzje:**
- DRY: Wspólny partial dla edytora (Cropper.js)
- Autoryzacja: X-Edit-Token header dla wszystkich operacji
- UX: Drag & drop dla dodawania zdjęć
- Puste galerie: Pozostają, cron usuwa po 3 dniach (istniejący mechanizm)
