package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	GalleryID    *int64
}

type Gallery struct {
	ID          int64
	Slug        string
	EditToken   string
	Title       string
	Description string
	UserID      *int64
	CreatedAt   int64
	UpdatedAt   int64
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
		slug CHAR(6) NOT NULL UNIQUE,
		display_name TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS galleries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug CHAR(4) NOT NULL UNIQUE,
		edit_token CHAR(32) NOT NULL,
		title TEXT,
		description TEXT,
		user_id INTEGER,
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

	CREATE INDEX IF NOT EXISTS idx_users_slug ON users(slug);
	CREATE INDEX IF NOT EXISTS idx_images_created ON images(created_at);
	CREATE INDEX IF NOT EXISTS idx_images_gallery ON images(gallery_id);
	CREATE INDEX IF NOT EXISTS idx_images_user ON images(user_id);
	CREATE INDEX IF NOT EXISTS idx_images_slug ON images(slug);
	CREATE INDEX IF NOT EXISTS idx_galleries_slug ON galleries(slug);
	CREATE INDEX IF NOT EXISTS idx_galleries_user ON galleries(user_id);
	CREATE INDEX IF NOT EXISTS idx_galleries_edit ON galleries(edit_token);
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) InsertImage(img *Image) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO images (slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.Slug, img.OriginalName, img.MimeType, img.FileSize, img.Width, img.Height, img.UserID, img.CreatedAt, img.AccessedAt, img.GalleryID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetImageBySlug(slug string) (*Image, error) {
	img := &Image{}
	err := db.conn.QueryRow(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id
		FROM images WHERE slug = ?`, slug).Scan(
		&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
		&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.GalleryID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return img, err
}

func (db *DB) TouchImageBySlug(slug string) error {
	_, err := db.conn.Exec("UPDATE images SET accessed_at = ? WHERE slug = ?", time.Now().Unix(), slug)
	return err
}

func (db *DB) InsertGallery(g *Gallery) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO galleries (slug, edit_token, title, description, user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.Slug, g.EditToken, g.Title, g.Description, g.UserID, g.CreatedAt, g.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetGalleryBySlug(slug string) (*Gallery, error) {
	g := &Gallery{}
	err := db.conn.QueryRow(`
		SELECT id, slug, edit_token, title, description, user_id, created_at, updated_at
		FROM galleries WHERE slug = ?`, slug).Scan(&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

// User functions (for V2 SSO)
func (db *DB) InsertUser(u *User) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO users (slug, display_name, created_at)
		VALUES (?, ?, ?)`,
		u.Slug, u.DisplayName, u.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetUserBySlug(slug string) (*User, error) {
	u := &User{}
	err := db.conn.QueryRow(`
		SELECT id, slug, display_name, created_at
		FROM users WHERE slug = ?`, slug).Scan(&u.ID, &u.Slug, &u.DisplayName, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetGalleryImages(galleryID int64) ([]*Image, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id
		FROM images WHERE gallery_id = ? ORDER BY created_at`, galleryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
			&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.GalleryID); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (db *DB) DeleteImageBySlug(slug string) error {
	_, err := db.conn.Exec("DELETE FROM images WHERE slug = ?", slug)
	return err
}

func (db *DB) GetOldestImages(limit int) ([]*Image, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id
		FROM images ORDER BY created_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
			&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.GalleryID); err != nil {
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
