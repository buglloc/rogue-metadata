package idpsrv

import (
	"net/http"
)

func NewServer(addr string, handler http.Handler) (*http.Server, error) {
	wrappedHandler := &HandlerWrapper{
		h: handler,
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: wrappedHandler,
	}

	return srv, nil
}
