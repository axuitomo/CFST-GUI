package main

import (
	"github.com/axuitomo/CFST-GUI/internal/adapters/host"
	frontendassets "github.com/axuitomo/CFST-GUI/packages/frontend"
)

func runtimeResources() host.Resources {
	return host.Resources{
		FrontendAssets: frontendassets.Dist,
	}
}
