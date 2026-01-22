package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type User struct {
	ID          int64
	Slug        string
	DisplayName string
	CreatedAt   int64
	UpdatedAt   int64
}

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
	Edited       bool
	EditToken    string `json:"edit_token,omitempty"`
}

type Gallery struct {
	ID          int64
	Slug        string
	EditToken   string
	Title       string
	Description string
	UserID      *int64
	ExternalID  *string
	CreatedAt   int64
	UpdatedAt   int64
}

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt int64
	CreatedAt int64
}

func NewDB(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "dajtu.db")
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug CHAR(4) NOT NULL UNIQUE,
		display_name TEXT NOT NULL UNIQUE,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS galleries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug CHAR(4) NOT NULL UNIQUE,
		edit_token CHAR(32) NOT NULL,
		title TEXT,
		description TEXT,
		user_id INTEGER,
		external_id TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug CHAR(5) NOT NULL UNIQUE,
		original_name TEXT,
		mime_type TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		width INTEGER,
		height INTEGER,
		user_id INTEGER,
		created_at INTEGER NOT NULL,
		accessed_at INTEGER NOT NULL,
		gallery_id INTEGER,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
		FOREIGN KEY (gallery_id) REFERENCES galleries(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token CHAR(64) PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_users_slug ON users(slug);
	CREATE INDEX IF NOT EXISTS idx_images_created ON images(created_at);
	CREATE INDEX IF NOT EXISTS idx_images_gallery ON images(gallery_id);
	CREATE INDEX IF NOT EXISTS idx_images_user ON images(user_id);
	CREATE INDEX IF NOT EXISTS idx_images_slug ON images(slug);
	CREATE INDEX IF NOT EXISTS idx_galleries_slug ON galleries(slug);
	CREATE INDEX IF NOT EXISTS idx_galleries_user ON galleries(user_id);
	CREATE INDEX IF NOT EXISTS idx_galleries_edit ON galleries(edit_token);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}

	// Migration: add external_id column if missing
	var hasColumn bool
	rows, err := db.conn.Query("PRAGMA table_info(galleries)")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "external_id" {
			hasColumn = true
			break
		}
	}
	if !hasColumn {
		if _, err := db.conn.Exec("ALTER TABLE galleries ADD COLUMN external_id TEXT"); err != nil {
			return err
		}
		if _, err := db.conn.Exec("CREATE INDEX IF NOT EXISTS idx_galleries_external ON galleries(external_id)"); err != nil {
			return err
		}
	}

	// Migration: add downloads column if missing
	_, err = db.conn.Exec(`ALTER TABLE images ADD COLUMN downloads INTEGER NOT NULL DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate downloads: %w", err)
	}

	// Migration: add edited column if missing
	_, err = db.conn.Exec(`ALTER TABLE images ADD COLUMN edited INTEGER NOT NULL DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate edited: %w", err)
	}

	// Migration: add edit_token column if missing
	_, err = db.conn.Exec(`ALTER TABLE images ADD COLUMN edit_token TEXT`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate edit_token: %w", err)
	}

	return nil
}

func (db *DB) InsertImage(img *Image) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO images (slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, downloads, gallery_id, edited, edit_token)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.Slug, img.OriginalName, img.MimeType, img.FileSize, img.Width, img.Height, img.UserID, img.CreatedAt, img.AccessedAt, img.Downloads, img.GalleryID, img.Edited, img.EditToken)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetImageBySlug(slug string) (*Image, error) {
	img := &Image{}
	err := db.conn.QueryRow(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, downloads, gallery_id, edited, edit_token
		FROM images WHERE slug = ?`, slug).Scan(
		&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
		&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.Downloads, &img.GalleryID, &img.Edited, &img.EditToken)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return img, err
}

func (db *DB) TouchImageBySlug(slug string) error {
	_, err := db.conn.Exec("UPDATE images SET accessed_at = ? WHERE slug = ?", time.Now().Unix(), slug)
	return err
}

func (db *DB) IncrementDownloads(slug string) error {
	_, err := db.conn.Exec(`UPDATE images SET downloads = downloads + 1 WHERE slug = ?`, slug)
	return err
}

func (db *DB) InsertGallery(g *Gallery) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO galleries (slug, edit_token, title, description, user_id, external_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.Slug, g.EditToken, g.Title, g.Description, g.UserID, g.ExternalID, g.CreatedAt, g.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetGalleryBySlug(slug string) (*Gallery, error) {
	g := &Gallery{}
	err := db.conn.QueryRow(`
		SELECT id, slug, edit_token, title, description, user_id, external_id, created_at, updated_at
		FROM galleries WHERE slug = ?`, slug).Scan(&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (db *DB) GetGalleryByID(id int64) (*Gallery, error) {
	g := &Gallery{}
	err := db.conn.QueryRow(`
		SELECT id, slug, edit_token, title, description, user_id, external_id, created_at, updated_at
		FROM galleries WHERE id = ?`, id).Scan(&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (db *DB) GetGalleryByExternalID(externalID string) (*Gallery, error) {
	g := &Gallery{}
	err := db.conn.QueryRow(`
		SELECT id, slug, edit_token, title, description, user_id, external_id, created_at, updated_at
		FROM galleries WHERE external_id = ?`, externalID).Scan(&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

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

func (db *DB) GetUserGalleries(userID int64) ([]*Gallery, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, edit_token, title, description, user_id, external_id, created_at, updated_at
		FROM galleries WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var galleries []*Gallery
	for rows.Next() {
		g := &Gallery{}
		if err := rows.Scan(&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		galleries = append(galleries, g)
	}
	return galleries, rows.Err()
}

// User functions (for V2 SSO)
func (db *DB) InsertUser(u *User) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO users (slug, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?)`,
		u.Slug, u.DisplayName, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetUserBySlug(slug string) (*User, error) {
	u := &User{}
	err := db.conn.QueryRow(`
		SELECT id, slug, display_name, created_at, COALESCE(updated_at, created_at)
		FROM users WHERE slug = ?`, slug).Scan(&u.ID, &u.Slug, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := db.conn.QueryRow(`
		SELECT id, slug, display_name, created_at, COALESCE(updated_at, created_at)
		FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Slug, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetOrCreateBratUser(pseudonim string) (*User, error) {
	now := time.Now().Unix()

	var user User
	err := db.conn.QueryRow(`
		SELECT id, slug, display_name, created_at, COALESCE(updated_at, created_at)
		FROM users WHERE display_name = ?
	`, pseudonim).Scan(&user.ID, &user.Slug, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err == nil {
		_, _ = db.conn.Exec(`UPDATE users SET updated_at = ? WHERE id = ?`, now, user.ID)
		user.UpdatedAt = now
		return &user, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	slug, err := generateUniqueUserSlug(db, 4)
	if err != nil {
		return nil, err
	}
	res, err := db.conn.Exec(`
		INSERT INTO users (slug, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, slug, pseudonim, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	return &User{
		ID:          id,
		Slug:        slug,
		DisplayName: pseudonim,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func generateUniqueUserSlug(db *DB, length int) (string, error) {
	for i := 0; i < 20; i++ {
		slug := GenerateSlug(length)
		exists, err := db.SlugExists("users", slug)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
	}
	return "", errors.New("failed to generate unique user slug")
}

func (db *DB) CreateSession(userID int64, durationDays int) (*Session, error) {
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	expiresAt := now + int64(durationDays*24*60*60)
	tokenHash := hashSessionToken(token)

	_, err = db.conn.Exec(
		`INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, expiresAt, now,
	)
	if err != nil {
		return nil, err
	}

	return &Session{Token: token, UserID: userID, ExpiresAt: expiresAt, CreatedAt: now}, nil
}

func (db *DB) GetSession(token string) (*Session, error) {
	var s Session
	tokenHash := hashSessionToken(token)
	err := db.conn.QueryRow(
		`SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ? AND expires_at > ?`,
		tokenHash, time.Now().Unix(),
	).Scan(&s.Token, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) DeleteSession(token string) error {
	tokenHash := hashSessionToken(token)
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE token = ?`, tokenHash)
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

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (db *DB) GetGalleryImages(galleryID int64) ([]*Image, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, downloads, gallery_id, edited, edit_token
		FROM images WHERE gallery_id = ? ORDER BY created_at`, galleryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
			&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.Downloads, &img.GalleryID, &img.Edited, &img.EditToken); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (db *DB) GetGalleryImagesPaginated(galleryID int64, limit, offset int) ([]*Image, int, error) {
	var total int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM images WHERE gallery_id = ?`, galleryID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.conn.Query(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, downloads, gallery_id, edited, edit_token
		FROM images WHERE gallery_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, galleryID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
			&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.Downloads, &img.GalleryID, &img.Edited, &img.EditToken); err != nil {
			return nil, 0, err
		}
		images = append(images, img)
	}

	return images, total, rows.Err()
}

func (db *DB) DeleteImageBySlug(slug string) error {
	_, err := db.conn.Exec("DELETE FROM images WHERE slug = ?", slug)
	return err
}

func (db *DB) GetOldestImages(limit int) ([]*Image, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, downloads, gallery_id, edited, edit_token
		FROM images ORDER BY accessed_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
			&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.Downloads, &img.GalleryID, &img.Edited, &img.EditToken); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (db *DB) GetTotalSize() (int64, error) {
	var total int64
	err := db.conn.QueryRow("SELECT COALESCE(SUM(file_size), 0) FROM images").Scan(&total)
	return total, err
}

func (db *DB) SlugExists(table, slug string) (bool, error) {
	var count int
	err := db.conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE slug = ?", table), slug).Scan(&count)
	return count > 0, err
}

func (db *DB) Close() error {
	return db.conn.Close()
}

type Stats struct {
	TotalImages    int64
	TotalGalleries int64
	TotalUsers     int64
	DiskUsageBytes int64
}

type GalleryAdmin struct {
	ID         int64
	Slug       string
	Title      string
	UserID     *int64
	CreatedAt  int64
	ImageCount int
	OwnerName  string
	OwnerSlug  string
}

type ImageAdmin struct {
	ID           int64
	Slug         string
	OriginalName string
	FileSize     int64
	Downloads    int64
	CreatedAt    int64
	AccessedAt   int64
	OwnerName    string
	OwnerSlug    string
	GallerySlug  string
}

func (db *DB) GetStats() (*Stats, error) {
	var stats Stats

	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM images`).Scan(&stats.TotalImages); err != nil {
		return nil, err
	}
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM galleries`).Scan(&stats.TotalGalleries); err != nil {
		return nil, err
	}
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers); err != nil {
		return nil, err
	}
	if err := db.conn.QueryRow(`SELECT COALESCE(SUM(file_size), 0) FROM images`).Scan(&stats.DiskUsageBytes); err != nil {
		return nil, err
	}

	return &stats, nil
}

func (db *DB) ListUsers(limit, offset int) ([]*User, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, display_name, created_at, COALESCE(updated_at, created_at)
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
		if err := rows.Scan(&u.ID, &u.Slug, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GenerateUniqueSlug generuje unikalny slug dla tabeli
func (db *DB) GenerateUniqueSlug(table string, length int) string {
	validTables := map[string]bool{"images": true, "galleries": true, "users": true}
	if !validTables[table] {
		return GenerateSlug(length)
	}

	const maxRetries = 100
	for attempt := 0; attempt < maxRetries; attempt++ {
		candidates := make([]string, 20)
		for i := range candidates {
			candidates[i] = GenerateSlug(length)
		}
		for _, slug := range candidates {
			exists, err := db.SlugExists(table, slug)
			if err == nil && !exists {
				return slug
			}
		}
	}
	panic("GenerateUniqueSlug: exceeded max retries, possible DB issue")
}

func (db *DB) ListGalleriesAdmin(limit, offset int) ([]*GalleryAdmin, error) {
	rows, err := db.conn.Query(`
		SELECT g.id, g.slug, g.title, g.user_id, g.created_at,
		       COUNT(i.id) as image_count,
		       COALESCE(u.display_name, '') as owner_name,
		       COALESCE(u.slug, '') as owner_slug
		FROM galleries g
		LEFT JOIN images i ON i.gallery_id = g.id
		LEFT JOIN users u ON u.id = g.user_id
		GROUP BY g.id
		ORDER BY g.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
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

func (db *DB) GetImagesByGallery(galleryID int64) ([]*Image, error) {
	rows, err := db.conn.Query(`SELECT id, slug FROM images WHERE gallery_id = ?`, galleryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (db *DB) DeleteGalleryByID(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM galleries WHERE id = ?`, id)
	return err
}

func (db *DB) ListImagesAdmin(limit, offset int) ([]*ImageAdmin, error) {
	rows, err := db.conn.Query(`
		SELECT i.id, i.slug, i.original_name, i.file_size, i.downloads, i.created_at, i.accessed_at,
		       COALESCE(u.display_name, '') as owner_name,
		       COALESCE(u.slug, '') as owner_slug,
		       COALESCE(g.slug, '') as gallery_slug
		FROM images i
		LEFT JOIN users u ON u.id = i.user_id
		LEFT JOIN galleries g ON g.id = i.gallery_id
		ORDER BY i.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*ImageAdmin
	for rows.Next() {
		img := &ImageAdmin{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.FileSize, &img.Downloads, &img.CreatedAt, &img.AccessedAt, &img.OwnerName, &img.OwnerSlug, &img.GallerySlug); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

func (db *DB) GetImageByID(id int64) (*Image, error) {
	row := db.conn.QueryRow(`SELECT id, slug FROM images WHERE id = ?`, id)
	img := &Image{}
	err := row.Scan(&img.ID, &img.Slug)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (db *DB) DeleteImageByID(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM images WHERE id = ?`, id)
	return err
}

func (db *DB) MarkImageEdited(slug string) error {
	_, err := db.conn.Exec("UPDATE images SET edited = 1 WHERE slug = ?", slug)
	return err
}

func (db *DB) UnmarkImageEdited(slug string) error {
	_, err := db.conn.Exec("UPDATE images SET edited = 0 WHERE slug = ?", slug)
	return err
}

func (db *DB) UpdateImageMetadata(slug string, width, height int, fileSize int64) error {
	_, err := db.conn.Exec(
		"UPDATE images SET width = ?, height = ?, file_size = ? WHERE slug = ?",
		width, height, fileSize, slug)
	return err
}

func (db *DB) UpdateGalleryTitle(id int64, title string) error {
	_, err := db.conn.Exec("UPDATE galleries SET title = ?, updated_at = ? WHERE id = ?",
		title, time.Now().Unix(), id)
	return err
}

func (db *DB) AddImageToGallery(galleryID, imageID int64) error {
	res, err := db.conn.Exec(
		"UPDATE images SET gallery_id = ? WHERE id = ? AND gallery_id IS NULL",
		galleryID, imageID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("image already in gallery or not found")
	}
	return nil
}
