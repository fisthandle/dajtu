package handler

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/logging"
	"dajtu/internal/middleware"
	"dajtu/internal/storage"
)

//go:embed templates/*
var templates embed.FS

type GalleryHandler struct {
	cfg         *config.Config
	db          *storage.DB
	fs          *storage.Filesystem
	galleryTmpl *template.Template
	indexTmpl   *template.Template
}

func NewGalleryHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem) *GalleryHandler {
	galleryTmpl := template.Must(template.ParseFS(templates, "templates/gallery.html", "templates/partials/*.html"))
	indexTmpl := template.Must(template.ParseFS(templates, "templates/index.html", "templates/partials/*.html"))
	return &GalleryHandler{cfg: cfg, db: db, fs: fs, galleryTmpl: galleryTmpl, indexTmpl: indexTmpl}
}

// GET / - index page with upload form
func (h *GalleryHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	var userData map[string]any
	var isAdmin bool
	user := middleware.GetUser(r)
	if user != nil {
		userData = map[string]any{
			"Slug":        user.Slug,
			"DisplayName": user.DisplayName,
		}
		isAdmin = slices.Contains(h.cfg.AdminNicks, user.DisplayName)
	}
	canUpload := h.cfg.PublicUpload || user != nil
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.indexTmpl.ExecuteTemplate(w, "index.html", map[string]any{
		"User":      userData,
		"IsAdmin":   isAdmin,
		"CanUpload": canUpload,
		"Welcome":   r.URL.Query().Get("welcome") == "1",
	}); err != nil {
		logging.Get("gallery").Printf("index template error: %v", err)
	}
}

type GalleryCreateResponse struct {
	Slug      string           `json:"slug"`
	URL       string           `json:"url"`
	EditToken string           `json:"edit_token"`
	Images    []UploadResponse `json:"images"`
}

// POST /gallery - create new gallery with images
func (h *GalleryHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.cfg.PublicUpload {
		user := middleware.GetUser(r)
		if user == nil {
			logging.Get("gallery").Printf("gallery.Create: public upload disabled, user not logged in")
			jsonError(w, "Galerie mogą tworzyć tylko zweryfikowani użytkownicy", http.StatusForbidden)
			return
		}
		logging.Get("gallery").Printf("gallery.Create: public upload disabled but user %s is logged in", user.DisplayName)
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024*10) // 10x for multiple files

	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024 * 10); err != nil {
		logging.Get("gallery").Printf("gallery.Create: parse error: %v", err)
		jsonError(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	existingImageSlug := r.FormValue("existing_image")
	files := r.MultipartForm.File["files"]
	logging.Get("gallery").Printf("gallery.Create: method=%s existing_image=%s has_edit_token=%v files=%d", r.Method, existingImageSlug, r.Header.Get("X-Edit-Token") != "", len(files))

	if existingImageSlug == "" && len(files) == 0 {
		logging.Get("gallery").Printf("gallery.Create: no files provided existing_image=%s", existingImageSlug)
		jsonError(w, "no files provided", http.StatusBadRequest)
		return
	}

	// Create gallery (4-char slug)
	gallerySlug := h.db.GenerateUniqueSlug("galleries", 4)
	editToken, _ := generateEditToken()
	now := time.Now().Unix()

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

	galleryID, err := h.db.InsertGallery(gallery)
	if err != nil {
		logging.Get("gallery").Printf("gallery.Create: db insert error: %v", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	baseURL := getBaseURL(h.cfg, r)

	var uploadedImages []UploadResponse

	// Handle existing image if provided
	if existingImageSlug != "" {
		editToken := r.Header.Get("X-Edit-Token")
		if editToken == "" {
			editToken = r.FormValue("edit_token")
		}

		existingImage, err := h.db.GetImageBySlug(existingImageSlug)
		if err != nil || existingImage == nil {
			if delErr := h.db.DeleteGalleryByID(galleryID); delErr != nil {
				logging.Get("gallery").Printf("failed to rollback gallery %d: %v", galleryID, delErr)
			}
			logging.Get("gallery").Printf("gallery.Create: existing image not found slug=%s", existingImageSlug)
			jsonError(w, "image not found", http.StatusNotFound)
			return
		}

		if existingImage.EditToken != editToken {
			if delErr := h.db.DeleteGalleryByID(galleryID); delErr != nil {
				logging.Get("gallery").Printf("failed to rollback gallery %d: %v", galleryID, delErr)
			}
			logging.Get("gallery").Printf("gallery.Create: unauthorized existing_image=%s has_edit_token=%v", existingImageSlug, editToken != "")
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if err := h.db.AddImageToGallery(galleryID, existingImage.ID); err != nil {
			if delErr := h.db.DeleteGalleryByID(galleryID); delErr != nil {
				logging.Get("gallery").Printf("failed to rollback gallery %d: %v", galleryID, delErr)
			}
			logging.Get("gallery").Printf("gallery.Create: add image to gallery failed: %v", err)
			jsonError(w, "database error", http.StatusInternalServerError)
			return
		}

		sizes := make(map[string]string)
		sizes["original"] = buildImageURL(baseURL, existingImageSlug, "original")
		sizes["1200"] = buildImageURL(baseURL, existingImageSlug, "1200")
		sizes["thumb"] = buildImageURL(baseURL, existingImageSlug, "thumb")

		uploadedImages = append(uploadedImages, UploadResponse{
			Slug:  existingImageSlug,
			URL:   sizes["original"],
			Sizes: sizes,
		})
	}

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
		format, data, err := image.ValidateAndDetect(file, maxSize)
		file.Close()
		if err != nil {
			continue // skip invalid files
		}

		slug := h.db.GenerateUniqueSlug("images", 5)

		var originalSize int64
		if h.cfg.KeepOriginalFormat {
			size, err := h.fs.SaveOriginal(slug, "original", data, string(format))
			if err != nil {
				logging.Get("gallery").Printf("warning: failed to save original: %v", err)
			} else {
				originalSize = size
			}
		}

		results, err := image.Process(data)
		if err != nil {
			h.fs.Delete(slug)
			continue
		}

		totalSize := originalSize
		for _, res := range results {
			if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
				h.fs.Delete(slug)
				continue
			}
			totalSize += int64(len(res.Data))
		}

		img := &storage.Image{
			Slug:         slug,
			OriginalName: fileHeader.Filename,
			MimeType:     string(format),
			FileSize:     totalSize,
			Width:        results[0].Width,
			Height:       results[0].Height,
			CreatedAt:    now,
			AccessedAt:   now,
			GalleryID:    &galleryID,
		}

		if _, err := h.db.InsertImage(img); err != nil {
			h.fs.Delete(slug)
			continue
		}

		sizes := make(map[string]string)
		for _, res := range results {
			sizes[res.Name] = buildImageURL(baseURL, slug, res.Name)
		}

		uploadedImages = append(uploadedImages, UploadResponse{
			Slug:  slug,
			URL:   sizes["original"],
			Sizes: sizes,
		})
	}

	if len(uploadedImages) == 0 {
		if delErr := h.db.DeleteGalleryByID(galleryID); delErr != nil {
			logging.Get("gallery").Printf("failed to rollback gallery %d: %v", galleryID, delErr)
		}
		logging.Get("gallery").Printf("gallery.Create: no valid images uploaded")
		jsonError(w, "no valid images uploaded", http.StatusBadRequest)
		return
	}

	resp := GalleryCreateResponse{
		Slug:      gallerySlug,
		URL:       baseURL + "/g/" + gallerySlug,
		EditToken: editToken,
		Images:    uploadedImages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /gallery/:slug/add - add images to gallery
func (h *GalleryHandler) AddImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract gallery slug from path: /gallery/XXXX/add
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "add" {
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
		logging.Get("gallery").Printf("gallery.AddImages: invalid edit token slug=%s", gallerySlug)
		jsonError(w, "invalid edit token", http.StatusForbidden)
		return
	}

	// Process same as Create but with existing gallery
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024*10)

	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024 * 10); err != nil {
		logging.Get("gallery").Printf("gallery.AddImages: parse error slug=%s: %v", gallerySlug, err)
		jsonError(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		logging.Get("gallery").Printf("gallery.AddImages: no files provided slug=%s", gallerySlug)
		jsonError(w, "no files provided", http.StatusBadRequest)
		return
	}
	logging.Get("gallery").Printf("gallery.AddImages: slug=%s files=%d", gallerySlug, len(files))

	baseURL := getBaseURL(h.cfg, r)

	now := time.Now().Unix()
	var uploadedImages []UploadResponse

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
		format, data, err := image.ValidateAndDetect(file, maxSize)
		file.Close()
		if err != nil {
			continue
		}

		slug := h.db.GenerateUniqueSlug("images", 5)

		var originalSize int64
		if h.cfg.KeepOriginalFormat {
			size, err := h.fs.SaveOriginal(slug, "original", data, string(format))
			if err != nil {
				logging.Get("gallery").Printf("warning: failed to save original: %v", err)
			} else {
				originalSize = size
			}
		}

		results, err := image.Process(data)
		if err != nil {
			h.fs.Delete(slug)
			continue
		}

		totalSize := originalSize
		for _, res := range results {
			if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
				h.fs.Delete(slug)
				continue
			}
			totalSize += int64(len(res.Data))
		}

		img := &storage.Image{
			Slug:         slug,
			OriginalName: fileHeader.Filename,
			MimeType:     string(format),
			FileSize:     totalSize,
			Width:        results[0].Width,
			Height:       results[0].Height,
			CreatedAt:    now,
			AccessedAt:   now,
			GalleryID:    &gallery.ID,
		}

		if _, err := h.db.InsertImage(img); err != nil {
			h.fs.Delete(slug)
			continue
		}

		sizes := make(map[string]string)
		for _, res := range results {
			sizes[res.Name] = buildImageURL(baseURL, slug, res.Name)
		}

		uploadedImages = append(uploadedImages, UploadResponse{
			Slug:  slug,
			URL:   sizes["original"],
			Sizes: sizes,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"added":      uploadedImages,
		"edit_token": gallery.EditToken,
	})
}

// DELETE /gallery/:slug/:img_slug - remove image from gallery
func (h *GalleryHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slugs from path: /gallery/XXXX/YYYYY
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	gallerySlug := parts[0]
	imageSlug := parts[1]

	// Verify edit token
	editToken := r.Header.Get("X-Edit-Token")
	if editToken == "" {
		r.ParseForm()
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

	// Verify image belongs to gallery
	img, err := h.db.GetImageBySlug(imageSlug)
	if err != nil || img == nil || img.GalleryID == nil || *img.GalleryID != gallery.ID {
		http.NotFound(w, r)
		return
	}

	// Delete from filesystem and database
	h.fs.Delete(imageSlug)
	h.db.DeleteImageBySlug(imageSlug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": imageSlug})
}

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

	if err := r.ParseMultipartForm(1024 * 1024); err != nil {
		r.ParseForm()
	}
	newTitle := r.FormValue("title")

	if err := h.db.UpdateGalleryTitle(gallery.ID, newTitle); err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"title": newTitle})
}

// GET /g/:slug - view gallery
func (h *GalleryHandler) View(w http.ResponseWriter, r *http.Request) {
	gallerySlug := strings.TrimPrefix(r.URL.Path, "/g/")
	if gallerySlug == "" {
		http.NotFound(w, r)
		return
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	const perPage = 100
	offset := (page - 1) * perPage

	images, total, err := h.db.GetGalleryImagesPaginated(gallery.ID, perPage, offset)
	if err != nil {
		http.Error(w, "error loading images", http.StatusInternalServerError)
		return
	}

	totalPages := (total + perPage - 1) / perPage

	baseURL := getBaseURL(h.cfg, r)

	type ImageData struct {
		Slug      string
		URL       string
		ThumbURL  string
		Width     int
		Height    int
		UpdatedAt int64
	}

	var imageData []ImageData
	for _, img := range images {
		imageData = append(imageData, ImageData{
			Slug:      img.Slug,
			URL:       baseURL + "/i/" + img.Slug + "/1200.webp",
			ThumbURL:  baseURL + "/i/" + img.Slug + "/thumb.webp",
			Width:     img.Width,
			Height:    img.Height,
			UpdatedAt: img.UpdatedAt,
		})
	}

	editToken := r.URL.Query().Get("edit")
	editMode := editToken != "" && editToken == gallery.EditToken

	data := map[string]any{
		"Title":       gallery.Title,
		"Description": gallery.Description,
		"Images":      imageData,
		"BaseURL":     baseURL,
		"Slug":        gallery.Slug,
		"EditToken":   editToken,
		"EditMode":    editMode,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"TotalImages": total,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"PrevPage":    page - 1,
		"NextPage":    page + 1,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.galleryTmpl.ExecuteTemplate(w, "gallery.html", data); err != nil {
		logging.Get("gallery").Printf("template error: %v", err)
	}
}
