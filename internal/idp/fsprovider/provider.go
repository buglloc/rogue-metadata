package fsprovider

import (
	"net/http"

	"github.com/buglloc/rogue-metadata/internal/idp"
)

func NewProvider(dir string) idp.Provider {
	return http.FileServer(http.Dir(dir))
}
