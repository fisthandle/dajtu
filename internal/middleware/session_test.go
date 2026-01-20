package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dajtu/internal/storage"
)

func testDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.NewDB(dir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionMiddleware_NoSession(t *testing.T) {
	db := testDB(t)
	mw := NewSessionMiddleware(db)

	var gotUser *storage.User
	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if gotUser != nil {
		t.Error("GetUser() should return nil when no session cookie")
	}
}

func TestSessionMiddleware_ValidSession(t *testing.T) {
	db := testDB(t)
	mw := NewSessionMiddleware(db)

	user, _ := db.GetOrCreateBratUser("testuser", 100)
	session, _ := db.CreateSession(user.ID, 30)

	var gotUser *storage.User
	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: session.Token})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if gotUser == nil {
		t.Fatal("GetUser() returned nil for valid session")
	}
	if gotUser.ID != user.ID {
		t.Errorf("GetUser().ID = %d, want %d", gotUser.ID, user.ID)
	}
	if gotUser.Slug != user.Slug {
		t.Errorf("GetUser().Slug = %s, want %s", gotUser.Slug, user.Slug)
	}
}

func TestSessionMiddleware_InvalidSession(t *testing.T) {
	db := testDB(t)
	mw := NewSessionMiddleware(db)

	var gotUser *storage.User
	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "invalid_token"})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if gotUser != nil {
		t.Error("GetUser() should return nil for invalid session token")
	}
}

func TestSessionMiddleware_EmptyToken(t *testing.T) {
	db := testDB(t)
	mw := NewSessionMiddleware(db)

	var gotUser *storage.User
	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: ""})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if gotUser != nil {
		t.Error("GetUser() should return nil for empty session token")
	}
}

func TestGetUser_NoContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	user := GetUser(req)

	if user != nil {
		t.Error("GetUser() should return nil when no user in context")
	}
}
