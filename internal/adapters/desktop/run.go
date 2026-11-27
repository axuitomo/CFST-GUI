package desktop

import "github.com/axuitomo/CFST-GUI/internal/adapters/host"

// Run starts the desktop GUI shell after enforcing removed-CLI and profiling guards.
func Run(args []string, resources host.Resources) {
	stopProfile := host.StartCPUProfileFromEnv()
	defer stopProfile()

	host.SetResources(resources)
	if dropped := host.HandleRemovedCLI(args); len(dropped) > 0 {
		host.SetStartupDroppedSources(dropped)
	}
	runGUI()
}
