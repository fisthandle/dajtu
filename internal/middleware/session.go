package middleware

import (
	"context"
	"net/http"

	"dajtu/internal/storage"
)

type contextKey string

const UserContextKey contextKey = "user"

type SessionMiddleware struct {
	db *storage.DB
}

func NewSessionMiddleware(db *storage.DB) *SessionMiddleware {
	return &SessionMiddleware{db: db}
}

// GetUser extracts user from request context (set by middleware)
func GetUser(r *http.Request) *storage.User {
	user, _ := r.Context().Value(UserContextKey).(*storage.User)
	return user
}

// Middleware adds user to context if valid session exists
func (m *SessionMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err == nil && cookie.Value != "" {
			session, err := m.db.GetSession(cookie.Value)
			if err == nil && session != nil {
				user, err := m.db.GetUserByID(session.UserID)
				if err == nil && user != nil {
					ctx := context.WithValue(r.Context(), UserContextKey, user)
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
