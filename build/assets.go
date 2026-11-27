package buildassets

import _ "embed"

// AppIcon stores the shared desktop application PNG icon.
//
//go:embed appicon.png
var AppIcon []byte
