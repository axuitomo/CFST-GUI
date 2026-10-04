package appcore

import configvalue "github.com/axuitomo/CFST-GUI/internal/config"

func intValue(value any, fallback int) int {
	return configvalue.Int(value, fallback)
}

func floatValue(value any, fallback float64) float64 {
	return configvalue.Float(value, fallback)
}

func boolValue(value any, fallback bool) bool {
	return configvalue.Bool(value, fallback)
}
