package idpsrv

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

var _ http.Handler = (*HandlerWrapper)(nil)

type HandlerWrapper struct {
	h http.Handler
}

func (h *HandlerWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("source", "http").Str("client", r.RemoteAddr).Str("uri", r.RequestURI).Msg("incoming request")
	h.h.ServeHTTP(w, r)
}
