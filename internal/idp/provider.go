package idp

import (
	"net/http"
)

type Provider interface {
	http.Handler
}
