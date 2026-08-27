package app

import (
	"path/filepath"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

func desktopColoDictionaryPaths() colodict.Paths {
	return colodict.DefaultPaths(filepath.Dir(desktopConfigFilePath()))
}
