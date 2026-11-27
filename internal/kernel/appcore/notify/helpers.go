package notify

import (
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/cloudflarecore"
)

type (
	CommandResult              = contracts.CommandResult
	UploadProviderReport       = contracts.UploadProviderReport
	UploadProviderResult       = contracts.UploadProviderResult
	UploadNotificationTopEntry = contracts.UploadNotificationTopEntry
	UploadNotificationFunnel   = contracts.UploadNotificationFunnel
	UploadNotification         = contracts.UploadNotification
)

var (
	mapValue              = config.Map
	firstNonNil           = config.FirstNonNil
	stringValue           = config.String
	boolValue             = config.Bool
	intValue              = config.Int
	floatValue            = config.Float
	stringSlice           = config.StringSlice
	uniqueNonEmptyStrings = config.UniqueNonEmptyStrings
	stringMap             = config.StringMap
	cloneAnyMap           = config.CloneMap
	IsMaskedSecret        = cloudflarecore.IsMaskedSecret

	dedupeStrings                          = DedupeStrings
	notificationTimeLabel                  = NotificationTimeLabel
	notificationDurationLabel              = NotificationDurationLabel
	contextWithTelegramNotificationTimeout = ContextWithTelegramNotificationTimeout
	notificationHTTPClient                 = NotificationHTTPClient
	telegramAPIBaseURL                     = TelegramAPIBaseURLForConfig
	uploadNotificationFunnel               = UploadNotificationFunnelForProbe

	LimitUploadRows                   = limitUploadRows
	CommandResultDataMap              = commandResultDataMap
	NormalizeUploadNotificationStatus = normalizeUploadNotificationStatus
)

type PostProbePushConfig struct {
	DNSEnabled    bool `json:"cloudflare_enabled"`
	GitHubEnabled bool `json:"github_enabled"`
}

func PostProbePushConfigFromSnapshot(snapshot map[string]any) PostProbePushConfig {
	raw := mapValue(snapshot["post_probe_push"])
	return PostProbePushConfig{
		DNSEnabled:    boolValue(firstNonNil(raw["cloudflare_enabled"], raw["cloudflareEnabled"], raw["dns_enabled"], raw["dnsEnabled"]), false),
		GitHubEnabled: boolValue(firstNonNil(raw["github_enabled"], raw["githubEnabled"]), false),
	}
}

func CloudflareProviderEnabledFromSnapshot(snapshot map[string]any) bool {
	cloudflare := mapValue(snapshot["cloudflare"])
	if boolValue(firstNonNil(cloudflare["enabled"], cloudflare["cloudflare_enabled"], cloudflare["cloudflareEnabled"]), false) {
		return true
	}
	return cloudflareRoutingProviderReady(snapshot)
}

func GitHubProviderEnabledFromSnapshot(snapshot map[string]any) bool {
	github := mapValue(snapshot["github"])
	if len(github) == 0 {
		export := mapValue(snapshot["export"])
		github = mapValue(export["github"])
	}
	return boolValue(firstNonNil(github["enabled"], github["github_enabled"], github["githubEnabled"]), false)
}

func cloudflareRoutingProviderReady(snapshot map[string]any) bool {
	cloudflare := mapValue(snapshot["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = snapshot
	}
	apiToken := strings.TrimSpace(stringValue(firstNonNil(cloudflare["api_token"], cloudflare["apiToken"]), ""))
	zoneID := strings.TrimSpace(stringValue(firstNonNil(cloudflare["zone_id"], cloudflare["zoneId"]), ""))
	if apiToken == "" || cloudflarecore.IsMaskedSecret(apiToken) || zoneID == "" {
		return false
	}

	routing := CloudflareRoutingConfigFromSnapshot(snapshot)
	if !routing.Enabled {
		return false
	}
	for _, rule := range routing.Rules {
		if rule.Enabled && strings.TrimSpace(rule.RecordName) != "" {
			return true
		}
	}
	return false
}
