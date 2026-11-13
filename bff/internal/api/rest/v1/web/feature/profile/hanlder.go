// bff\internal\api\rest\v1\web\feature\profile\hanlder.go
package profile

import (
	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/config"
)

type Handler struct {
	cfg     config.Config
	clients *ports.Clients
}

func New(cfg config.Config, clients *ports.Clients) *Handler {
	return &Handler{cfg: cfg, clients: clients}
}
