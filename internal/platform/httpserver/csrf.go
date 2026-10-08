package httpserver

import (
	"net/http"

	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

func OriginGuard(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
				if r.Header.Get("Origin") != cfg.PublicOrigin {
					_ = writeError(w, httperror.New(httperror.Forbidden, "origin_forbidden", "Request origin is not allowed.", nil))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
