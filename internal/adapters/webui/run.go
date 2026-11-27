package webui

import (
	"context"
	"fmt"
	"os"

	"github.com/axuitomo/CFST-GUI/internal/adapters/host"
)

// Run starts WebUI mode (or executes --healthcheck) with shared host resources.
func Run(args []string, resources host.Resources) {
	host.SetResources(resources)
	if shouldRunWebUIHealthcheck(args) {
		os.Exit(runWebUIHealthcheck(context.Background(), os.Getenv("CFST_WEBUI_ADDR"), webUIHealthcheckClient))
	}
	_ = host.HandleRemovedCLI(args)
	if err := runWebUI(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
