package webui

import (
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/adapters/host"
)

func setConfigHomeForTest(t *testing.T, root string) {
	t.Helper()
	host.ApplyTestConfigHome(t.Setenv, root)
}
