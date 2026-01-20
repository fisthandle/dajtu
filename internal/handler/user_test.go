package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dajtu/internal/testutil"
)

func TestUserHandler_NotFound(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)

	h := NewUserHandler(cfg, db)

	req := httptest.NewRequest("GET", "/u/unknown", nil)
	w := httptest.NewRecorder()

	h.View(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
