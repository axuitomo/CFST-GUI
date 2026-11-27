package host

import (
	"path/filepath"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/colodict"
)

func desktopColoDictionaryPaths() colodict.Paths {
	return colodict.DefaultPaths(filepath.Dir(DesktopConfigFilePath()))
}
