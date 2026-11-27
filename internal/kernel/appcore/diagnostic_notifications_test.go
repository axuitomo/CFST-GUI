package appcore

import (
	"bytes"
	"testing"
)

func TestDiagnosticPackageRedactsNotificationCredentials(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir(), ConfigFileName: "desktop-config.json"}})
	secrets := map[string]string{
		"smtp": "fixture-smtp-credential", "dingtalk": "fixture-dingtalk-credential",
		"wecom": "fixture-wecom-credential", "header": "fixture-header-credential",
		"url_path": "fixture-url-path-credential", "url_user": "fixture-url-user-credential",
	}
	snapshot := map[string]any{"notifications": map[string]any{
		"email": map[string]any{"password": secrets["smtp"]},
		"webhook": map[string]any{
			"dingtalk_secret": secrets["dingtalk"], "wecom_secret": secrets["wecom"],
			"headers":      map[string]any{"X-API-Key": secrets["header"]},
			"urls":         []string{"https://example.invalid/" + secrets["url_path"]},
			"dingtalk_url": "https://" + secrets["url_user"] + "@example.invalid/notify",
			"wecom_urls":   []string{"https://example.invalid/" + secrets["url_path"]},
		},
	}}
	if _, err := service.SaveConfig(snapshot); err != nil {
		t.Fatal(err)
	}
	body, _, err := service.BuildDiagnosticPackage()
	if err != nil {
		t.Fatal(err)
	}
	entries := unzipDiagnosticEntries(t, body)
	if len(entries["config/config-summary.json"]) == 0 {
		t.Fatal("missing config summary")
	}
	for name, raw := range entries {
		for kind, secret := range secrets {
			if bytes.Contains(raw, []byte(secret)) {
				t.Errorf("%s exposes the %s fixture credential", name, kind)
			}
		}
	}
	loaded, err := service.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	notifications := mapValue(loaded.Snapshot["notifications"])
	if mapValue(notifications["email"])["password"] != secrets["smtp"] {
		t.Fatal("diagnostic redaction changed the persisted email configuration")
	}
	if mapValue(mapValue(notifications["webhook"])["headers"])["X-API-Key"] != secrets["header"] {
		t.Fatal("diagnostic redaction changed the persisted webhook configuration")
	}
}
