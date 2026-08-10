package appcore

import (
	"path/filepath"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

func TestColoCommandsUseServicePaths(t *testing.T) {
	paths := colodict.DefaultPaths(filepath.Join(t.TempDir(), "colo"))
	service := NewService(ServiceOptions{ColoPaths: paths})
	status, handled := service.TryInvoke("colo.status", "{}")
	if !handled || !status.OK || status.Code != "COLO_DICTIONARY_STATUS_READY" {
		t.Fatalf("status = %#v, handled=%v", status, handled)
	}
	data, ok := status.Data.(colodict.Status)
	if !ok || data.ColoPath != paths.Colo || data.GeofeedPath != paths.Geofeed {
		t.Fatalf("status data = %#v", status.Data)
	}

	processed, handled := service.TryInvoke("colo.process", "{}")
	if !handled || processed.OK || processed.Code != "COLO_DICTIONARY_PROCESS_FAILED" {
		t.Fatalf("processed = %#v, handled=%v", processed, handled)
	}
}

func TestColoCommandRejectsInvalidPayload(t *testing.T) {
	service := NewService(ServiceOptions{})
	result, handled := service.TryInvoke("colo.update", "{")
	if !handled || result.OK || result.Code != "COLO_DICTIONARY_UPDATE_FAILED" {
		t.Fatalf("result = %#v, handled=%v", result, handled)
	}
}
