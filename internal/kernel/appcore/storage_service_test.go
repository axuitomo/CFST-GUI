package appcore

import (
	"errors"
	"reflect"
	"testing"
)

func TestServiceInvokeStorageUsesInjectedPlatformCapabilities(t *testing.T) {
	var setPayload map[string]any
	service := NewService(ServiceOptions{
		ArchiveStorageState: func() any { return map[string]any{"backend": "fallback"} },
		StorageCommands: StorageCommandHooks{
			Set: func(payload map[string]any) (StorageSetResult, error) {
				setPayload = payload
				return StorageSetResult{
					Migration: map[string]any{"copied": []string{"config.json"}},
					Storage:   map[string]any{"backend": "platform"},
				}, nil
			},
			Health: func(payload map[string]any) (StorageHealthResult, error) {
				if len(payload) != 0 {
					t.Fatalf("health payload = %#v", payload)
				}
				return StorageHealthResult{
					Health:  map[string]any{"writable": true},
					Storage: map[string]any{"backend": "platform"},
				}, nil
			},
		},
	})

	setResult := service.Invoke("storage.set", `{"display_name":"Documents"}`)
	if !setResult.OK || setResult.Code != "STORAGE_SET_DEPRECATED" || setPayload["display_name"] != "Documents" {
		t.Fatalf("storage set = %#v, payload = %#v", setResult, setPayload)
	}
	setData := mapValue(setResult.Data)
	if mapValue(setData["storage"])["backend"] != "platform" || !reflect.DeepEqual(mapValue(setData["migration"])["copied"], []string{"config.json"}) {
		t.Fatalf("storage set data = %#v", setData)
	}

	healthResult := service.Invoke("storage.health", `{}`)
	if !healthResult.OK || healthResult.Code != "STORAGE_HEALTH_READY" {
		t.Fatalf("storage health = %#v", healthResult)
	}
	healthData := mapValue(healthResult.Data)
	if !boolValue(mapValue(healthData["health"])["writable"], false) || mapValue(healthData["storage"])["backend"] != "platform" {
		t.Fatalf("storage health data = %#v", healthData)
	}
}

func TestServiceInvokeStorageReturnsStructuredErrors(t *testing.T) {
	service := NewService(ServiceOptions{StorageCommands: StorageCommandHooks{
		Set: func(map[string]any) (StorageSetResult, error) {
			return StorageSetResult{Storage: map[string]any{"writable": false}}, errors.New("write failed")
		},
		Health: func(map[string]any) (StorageHealthResult, error) {
			return StorageHealthResult{}, errors.New("health failed")
		},
	}})
	if result := service.Invoke("storage.set", `{}`); result.OK || result.Code != "STORAGE_SET_FAILED" {
		t.Fatalf("storage set error = %#v", result)
	}
	if result := service.Invoke("storage.health", `{}`); result.OK || result.Code != "STORAGE_HEALTH_FAILED" {
		t.Fatalf("storage health error = %#v", result)
	}
	if result := service.Invoke("storage.set", "{"); result.OK || result.Code != "COMMAND_PAYLOAD_INVALID" {
		t.Fatalf("storage malformed payload = %#v", result)
	}
}
