package middleware

import (
	"net/http"
	"slices"
)

type AdminMiddleware struct {
	adminNicks []string
}

func NewAdminMiddleware(adminNicks []string) *AdminMiddleware {
	return &AdminMiddleware{adminNicks: adminNicks}
}

func (m *AdminMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if !slices.Contains(m.adminNicks, user.DisplayName) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
