package mobileapi

import "github.com/axuitomo/CFST-GUI/internal/appcore"

type mobileTraceDiagnostics = appcore.TraceDiagnosticsCollector

func newMobileTraceDiagnostics(cfg probeConfig) *mobileTraceDiagnostics {
	return appcore.NewTraceDiagnostics(cfg.TraceColoMode, cfg.TraceURL)
}
