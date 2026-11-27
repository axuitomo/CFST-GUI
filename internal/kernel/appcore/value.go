package appcore

import configvalue "github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"

var (
	mapValue     = configvalue.Map
	firstNonNil  = configvalue.FirstNonNil
	firstPresent = configvalue.FirstPresent
	stringValue  = configvalue.String
	intValue     = configvalue.Int
	floatValue   = configvalue.Float
	boolValue    = configvalue.Bool
	cloneAnyMap  = configvalue.CloneMap
)
