package server

import "github.com/rifqif16/ecampus/backend/internal/api"

type API struct {
	Probes
}

var _ api.StrictServerInterface = API{}
