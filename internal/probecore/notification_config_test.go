package probecore

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNotificationConfigRoundTrip(t *testing.T) {
	notifications := map[string]any{
		"telegram": map[string]any{"api_base_url": "https://tg.example.invalid", "use_system_proxy": true},
		"webhook": map[string]any{
			"enabled": true, "urls": []string{"https://example.invalid/hook"},
			"dingtalk_url": "https://example.invalid/dingtalk", "dingtalk_secret": "test-secret",
			"wecom_urls": []string{"https://example.invalid/wecom"}, "wecom_corp_id": "test-corp",
			"wecom_agent_id": "42", "wecom_secret": "test-wecom-secret", "wecom_mobiles": []string{"13800000000"},
			"headers": map[string]any{"Authorization": "test-auth", "X-Custom": "test-value"}, "use_system_proxy": true,
		},
		"email": map[string]any{
			"enabled": true, "host": "smtp.example.invalid", "port": 465,
			"username": "test-user", "password": "test-password", "from": "sender@example.invalid",
			"to": []string{"recipient@example.invalid"}, "use_tls": false,
		},
	}
	for name, options := range map[string]ConfigSnapshotOptions{"desktop": DesktopConfigSnapshotOptions(), "mobile": MobileConfigSnapshotOptions()} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{"notifications": notifications})
			if err != nil {
				t.Fatal(err)
			}
			var input map[string]any
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			snapshot := SanitizeConfigSnapshot(input, options)
			got := testConfigMap(t, snapshot["notifications"])
			for _, section := range []string{"webhook", "email"} {
				actual, err := json.Marshal(got[section])
				if err != nil {
					t.Fatal(err)
				}
				expected, err := json.Marshal(notifications[section])
				if err != nil {
					t.Fatal(err)
				}
				if string(actual) != string(expected) {
					t.Fatalf("%s round-trip = %s, want %s", section, actual, expected)
				}
			}
			telegram := testConfigMap(t, got["telegram"])
			if telegram["use_system_proxy"] != true || telegram["api_base_url"] != "https://tg.example.invalid" {
				t.Fatalf("telegram round-trip = %#v, want proxy and custom API host kept", telegram)
			}
			if again := SanitizeConfigSnapshot(snapshot, options); !reflect.DeepEqual(again, snapshot) {
				t.Fatal("sanitization is not idempotent")
			}
			defaults := testConfigMap(t, SanitizeConfigSnapshot(nil, options)["notifications"])
			if testConfigMap(t, defaults["webhook"])["enabled"] != false || testConfigMap(t, defaults["email"])["use_tls"] != true {
				t.Fatalf("notification defaults = %#v", defaults)
			}
		})
	}
}

func TestNotificationConfigAliasesAndDynamicHeaders(t *testing.T) {
	headers := map[string]any{"X-Dynamic": "original"}
	snapshot := SanitizeConfigSnapshot(map[string]any{"notifications": map[string]any{
		"telegram": map[string]any{"apiHost": "https://alias.example.invalid", "useSystemProxy": true},
		"webhook":  map[string]any{"url": "https://example.invalid/one;https://example.invalid/two", "dingtalkUrl": "https://example.invalid/dingtalk", "dingtalkSecret": "test-secret", "useSystemProxy": true, "headers": headers, "unknown": "drop"},
		"email":    map[string]any{"useTLS": false, "to": "one@example.invalid;two@example.invalid"},
	}}, ConfigSnapshotOptions{})
	notifications := testConfigMap(t, snapshot["notifications"])
	webhook := testConfigMap(t, notifications["webhook"])
	telegram := testConfigMap(t, notifications["telegram"])
	if webhook["dingtalk_url"] != "https://example.invalid/dingtalk" || webhook["dingtalk_secret"] != "test-secret" || webhook["use_system_proxy"] != true || telegram["use_system_proxy"] != true || telegram["api_base_url"] != "https://alias.example.invalid" {
		t.Fatalf("notification aliases lost: %#v", notifications)
	}
	if !reflect.DeepEqual(webhook["urls"], []string{"https://example.invalid/one", "https://example.invalid/two"}) {
		t.Fatalf("webhook urls = %#v", webhook["urls"])
	}
	if _, ok := webhook["unknown"]; ok {
		t.Fatal("unknown configuration key survived")
	}
	gotHeaders := testConfigMap(t, webhook["headers"])
	if !reflect.DeepEqual(gotHeaders, headers) {
		t.Fatalf("headers = %#v", gotHeaders)
	}
	gotHeaders["X-Dynamic"] = "changed"
	if headers["X-Dynamic"] != "original" {
		t.Fatal("sanitized headers alias input")
	}
	email := testConfigMap(t, notifications["email"])
	if email["use_tls"] != false || !reflect.DeepEqual(email["to"], []string{"one@example.invalid", "two@example.invalid"}) {
		t.Fatalf("email aliases = %#v", email)
	}
}
