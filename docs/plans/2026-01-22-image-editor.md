# Image Editor Implementation Plan (Cropper.js v1)

**Goal:** Dodać edytor zdjęć z crop, rotate, resize, aspect ratio presets i undo/redo.

**Architecture:** Cropper.js v1 (stable) do UI i interakcji, bimg/libvips do finalnych transformacji server-side. Undo/redo przez własny history stack (Cropper.js v1 nie ma wbudowanego).

**Tech Stack:** Cropper.js v1.6.2 (CDN), bimg/libvips (backend).

---

## Ocena trudności

| Komponent | Trudność | Czas |
|-----------|----------|------|
| Cropper.js setup + UI | Łatwa | 30min |
| Rotate/flip buttons | Łatwa | 15min |
| Aspect ratio presets | Łatwa | 15min |
| Undo/redo stack | Średnia | 30-45min |
| Backend transformacje | Średnia | 1h |
| Integracja upload | Łatwa | 30min |
| **SUMA** | **Łatwa-Średnia** | **~3-4h** |

**Verdict:** Cropper.js daje touch support, zoom i crop UI out of box.

---

## Task 1: Cropper.js integration + basic UI

**Files:**
- Modify: `internal/handler/templates/index.html`

**Step 1: Add Cropper.js CSS and JS from CDN**

W `<head>` dodaj:

```html
<link rel="stylesheet" href="https://unpkg.com/cropperjs@1.6.2/dist/cropper.min.css">
```

Przed zamykającym `</body>` dodaj:

```html
<script src="https://unpkg.com/cropperjs@1.6.2/dist/cropper.min.js"></script>
```

**Step 2: Add editor CSS**

W sekcji `<style>` dodaj:

```css
.editor-container {
    margin-top: 20px;
    display: none;
}
.editor-container.active { display: block; }
.editor-toolbar {
    display: flex;
    gap: 8px;
    margin-bottom: 15px;
    flex-wrap: wrap;
    align-items: center;
}
.editor-toolbar button {
    padding: 8px 14px;
    border: 1px solid #333;
    border-radius: 6px;
    background: #1a1a1a;
    color: #fff;
    cursor: pointer;
    font-size: 0.85rem;
    transition: border-color 0.2s;
}
.editor-toolbar button:hover { border-color: #4a9eff; }
.editor-toolbar button:disabled { opacity: 0.4; cursor: not-allowed; }
.editor-toolbar button.active {
    border-color: #4a9eff;
    background: rgba(74, 158, 255, 0.1);
}
.editor-toolbar .separator {
    width: 1px;
    height: 24px;
    background: #333;
    margin: 0 4px;
}
.editor-toolbar .group {
    display: flex;
    gap: 4px;
}
.editor-canvas-wrap {
    width: 100%;
    max-width: 600px;
    max-height: 420px;
    background: #0a0a0a;
    border-radius: 8px;
    overflow: hidden;
}
.editor-canvas-wrap img {
    display: block;
    max-width: 100%;
}
.editor-info {
    margin-top: 10px;
    font-size: 0.85rem;
    color: #666;
}
.cropper-view-box {
    outline: 1px solid #4a9eff;
    outline-color: #4a9eff;
}
.cropper-line,
.cropper-point {
    background-color: #4a9eff;
}
.cropper-face {
    background-color: rgba(74, 158, 255, 0.08);
}
```

**Step 3: Add editor HTML (poza drop area)**

Po `</div>` zamykającym `.upload-area` dodaj:

```html
<div class="editor-container" id="editorContainer">
    <div class="editor-toolbar">
        <button id="editorUndo" disabled title="Cofnij (Ctrl+Z)">↶ Cofnij</button>
        <button id="editorRedo" disabled title="Ponów (Ctrl+Y)">↷ Ponów</button>
        <div class="separator"></div>
        <div class="group">
            <button id="editorRotateL" title="Obróć -90°">↺</button>
            <button id="editorRotateR" title="Obróć +90°">↻</button>
            <button id="editorFlipH" title="Odbij poziomo">⇆</button>
            <button id="editorFlipV" title="Odbij pionowo">⇅</button>
        </div>
        <div class="separator"></div>
        <div class="group">
            <button id="aspectFree" class="active">Free</button>
            <button id="aspect1x1">1:1</button>
            <button id="aspect16x9">16:9</button>
            <button id="aspect4x3">4:3</button>
        </div>
        <div class="separator"></div>
        <button id="editorReset" title="Resetuj wszystko">⟲ Reset</button>
    </div>
    <div class="editor-canvas-wrap" id="editorCanvasWrap">
        <img id="editorImage" src="" alt="Edit">
    </div>
    <div class="editor-info" id="editorInfo"></div>
</div>
```

---

## Task 2: Cropper initialization + transform controls + undo/redo

**Files:**
- Modify: `internal/handler/templates/index.html`

**Step 1: Add editor JavaScript**

W sekcji `<script>` dodaj logikę inicjalizacji:

```javascript
let cropper = null;
let historyStack = [];
let historyIndex = -1;
let currentAspectRatio = NaN;

function initEditor(file) {
    const editorImg = document.getElementById('editorImage');
    const editorDiv = document.getElementById('editorContainer');

    if (cropper) {
        cropper.destroy();
        cropper = null;
    }
    historyStack = [];
    historyIndex = -1;

    const objectUrl = URL.createObjectURL(file);
    editorImg.src = objectUrl;
    editorImg.onload = () => {
        URL.revokeObjectURL(objectUrl);
        cropper = new Cropper(editorImg, {
            viewMode: 1,
            autoCropArea: 1,
            background: false,
            checkOrientation: true,
            ready() {
                editorDiv.classList.add('active');
                saveState();
                updateEditorUI();
            },
            crop() { updateEditorUI(); }
        });
    };
}

function saveState() {
    if (!cropper) return;
    const state = {
        data: cropper.getData(true),
        aspectRatio: currentAspectRatio
    };
    historyStack = historyStack.slice(0, historyIndex + 1);
    historyStack.push(JSON.stringify(state));
    historyIndex++;
    updateUndoRedo();
}

function restoreState(index) {
    if (!cropper || index < 0 || index >= historyStack.length) return;
    const state = JSON.parse(historyStack[index]);
    currentAspectRatio = state.aspectRatio;
    cropper.setAspectRatio(currentAspectRatio);
    cropper.setData(state.data);
    historyIndex = index;
    updateUndoRedo();
    updateEditorUI();
}

function updateUndoRedo() {
    document.getElementById('editorUndo').disabled = historyIndex <= 0;
    document.getElementById('editorRedo').disabled = historyIndex >= historyStack.length - 1;
}

function updateEditorUI() {
    if (!cropper) return;
    const data = cropper.getData(true);
    const w = Math.round(data.width);
    const h = Math.round(data.height);
    document.getElementById('editorInfo').textContent = `Zaznaczenie: ${w} × ${h}px`;
}

// Transform controls
document.getElementById('editorRotateL').onclick = () => {
    cropper.rotate(-90);
    saveState();
};

document.getElementById('editorRotateR').onclick = () => {
    cropper.rotate(90);
    saveState();
};

document.getElementById('editorFlipH').onclick = () => {
    const scaleX = cropper.getData().scaleX || 1;
    cropper.scaleX(scaleX * -1);
    saveState();
};

document.getElementById('editorFlipV').onclick = () => {
    const scaleY = cropper.getData().scaleY || 1;
    cropper.scaleY(scaleY * -1);
    saveState();
};
```

**Step 2: Undo/redo + shortcuts**

Dodaj `undo`, `redo` i skróty klawiaturowe, np. przez `restoreState(historyIndex - 1)`.

---

## Task 3: Aspect ratio presets

**Files:**
- Modify: `internal/handler/templates/index.html`

**Step 1: Add aspect ratio controls**

```javascript
function setAspectRatio(ratio) {
    currentAspectRatio = ratio;
    cropper.setAspectRatio(ratio);
    // toggle button .active
}

document.getElementById('aspectFree').onclick = () => setAspectRatio(NaN);
document.getElementById('aspect1x1').onclick = () => setAspectRatio(1);
document.getElementById('aspect16x9').onclick = () => setAspectRatio(16/9);
document.getElementById('aspect4x3').onclick = () => setAspectRatio(4/3);
```

---

## Task 4: Integrate editor with upload flow

**Files:**
- Modify: `internal/handler/templates/index.html`

**Step 1: Show editor only for single image without title**

W `updatePreview()`:

```javascript
function updateEditorVisibility() {
    const files = fileInput.files;
    const hasSingle = files.length === 1 && !titleInput.value.trim();
    if (!hasSingle) {
        destroyEditor();
        return;
    }
    initEditor(files[0]);
}
```

**Step 2: Add getTransformParams**

```javascript
function getTransformParams() {
    if (!cropper) return null;
    const data = cropper.getData(true);
    return {
        rotation: Math.round(data.rotate || 0),
        flipH: data.scaleX === -1,
        flipV: data.scaleY === -1,
        cropX: Math.round(data.x),
        cropY: Math.round(data.y),
        cropW: Math.round(data.width),
        cropH: Math.round(data.height)
    };
}
```

**Step 3: Modify form submit to include transform params**

```javascript
if (files.length === 1 && !title) {
    formData.append('file', files[0]);
    const params = getTransformParams();
    if (params) {
        if (params.rotation) formData.append('rotation', params.rotation);
        if (params.flipH) formData.append('flipH', 'true');
        if (params.flipV) formData.append('flipV', 'true');
        formData.append('cropX', params.cropX);
        formData.append('cropY', params.cropY);
        formData.append('cropW', params.cropW);
        formData.append('cropH', params.cropH);
    }
    // POST /upload
}
```

---

## Task 5: Backend transform support

**Files:**
- Modify: `internal/image/processor.go`

**Step 1: Add TransformParams struct + ProcessWithTransform**

```go
// TransformParams holds image transformation parameters from the editor
// (crop coords are pixels on the rotated/flipped image)
type TransformParams struct {
    Rotation int
    FlipH    bool
    FlipV    bool
    CropX    int
    CropY    int
    CropW    int
    CropH    int
}

func (p TransformParams) HasTransforms() bool {
    return p.Rotation != 0 || p.FlipH || p.FlipV || (p.CropW > 0 && p.CropH > 0)
}

func ProcessWithTransform(data []byte, params TransformParams) ([]ProcessResult, error) {
    // Apply rotate/flip first, then crop (Cropper.js coordinates are for the transformed image)
    opts := bimg.Options{StripMetadata: true}
    // set opts.Rotate, opts.Flip, opts.Flop, opts.Top/Left/AreaWidth/AreaHeight
    transformed, err := bimg.NewImage(data).Process(opts)
    if err != nil {
        return nil, fmt.Errorf("transform: %w", err)
    }

    // Then generate standard sizes from the transformed image
    return processVariants(transformed, true)
}
```

---

## Task 6: Upload handler transform parsing

**Files:**
- Modify: `internal/handler/upload.go`

**Step 1: Add parseTransformParams helper**

```go
func parseTransformParams(r *http.Request) image.TransformParams {
    params := image.TransformParams{}
    // rotation, flipH/flipV, cropX/Y/W/H
    return params
}
```

**Step 2: Use ProcessWithTransform when needed**

```go
transformParams := parseTransformParams(r)
var results []image.ProcessResult
if transformParams.HasTransforms() {
    results, err = image.ProcessWithTransform(data, transformParams)
} else {
    results, err = image.Process(data)
}
```

---

## Task 7: Test locally

1. `go build ./...`
2. `go test ./...`
3. Uruchom aplikację i sprawdź w przeglądarce: crop, rotate, flip, aspect, undo/redo.

---

## Summary

| Task | Component | LOC estimate |
|------|-----------|--------------|
| 1 | Cropper.js setup + CSS | ~80 |
| 2 | Init + rotate/flip/undo | ~120 |
| 3 | Aspect ratio presets | ~30 |
| 4 | Upload integration | ~60 |
| 5 | Backend transforms | ~80 |
| 6 | Upload handler parsing | ~40 |
| 7 | Test | - |
| **TOTAL** | | **~410 LOC** |

### Zyski z Cropper.js vs ręczny Canvas

| Feature | Cropper.js | Wartość |
|---------|------------|---------|
| Touch/mobile zoom | ✅ Out of box | Nie trzeba pisać |
| Pinch gesture | ✅ Out of box | Nie trzeba pisać |
| Crop drag handles | ✅ Out of box | -100 LOC |
| Zoom wheel | ✅ Out of box | Nie trzeba pisać |
| Grid overlay | ✅ Wbudowany | Nie trzeba pisać |
