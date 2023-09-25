package proxyprovider

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"

	"github.com/rs/zerolog/log"

	"github.com/buglloc/rogue-metadata/internal/idp"
)

func NewProvider(upstream string) idp.Provider {
	l := log.With().Str("source", "idp.proxy").Logger()

	return &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = upstream
			r.Host = upstream
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			l.Error().Err(err).Str("url", r.URL.String()).Msg("request failed")
			w.WriteHeader(http.StatusBadGateway)
		},
		ModifyResponse: func(w *http.Response) error {
			body, err := io.ReadAll(w.Body)
			_ = w.Body.Close()
			if err != nil {
				return err
			}

			w.Body = io.NopCloser(bytes.NewBuffer(body))
			return nil
		},
	}
}
