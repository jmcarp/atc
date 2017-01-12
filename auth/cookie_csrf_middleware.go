package auth

import (
	"net/http"

	"github.com/gorilla/csrf"
)

func CookieCSRFMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(CookieName); err != nil {
			r = csrf.UnsafeSkipCheck(r)
		}
		handler.ServeHTTP(w, r)
	})
}
