package auth

import (
	"net/http"

	"github.com/gorilla/csrf"
)

func AuthCSRFMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			r = csrf.UnsafeSkipCheck(r)
		}
		handler.ServeHTTP(w, r)
	})
}
