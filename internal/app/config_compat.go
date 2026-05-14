package app

import "github.com/axuitomo/CFST-GUI/internal/probecore"

func sanitizeDesktopConfigSnapshot(input map[string]any) map[string]any {
	return probecore.SanitizeConfigSnapshot(input, desktopConfigSnapshotOptions())
}

func desktopConfigSnapshotOptions() probecore.ConfigSnapshotOptions {
	return probecore.ConfigSnapshotOptions{
		CloudflareTTL:                defaultCloudflareTTL,
		DefaultSourceIPLimit:         defaultDesktopSourceIPLimit,
		GitHubBranch:                 defaultGitHubExportBranch,
		GitHubCommitMessageTemplate:  defaultGitHubExportCommitMessageTemplate,
		GitHubOwner:                  defaultGitHubExportOwner(),
		GitHubPathTemplate:           defaultGitHubExportPathTemplate,
		GitHubRepo:                   defaultGitHubExportRepo(),
		IncludePortPolicy:            true,
		IncludeSchedulerWorkflow:     true,
		IncludeTheme:                 true,
		PortPolicy:                   defaultPortPolicy,
		SchedulerConfigSource:        defaultSchedulerConfigSource,
		SchedulerProfileAction:       defaultSchedulerProfileAction,
		SchedulerSourceProfileAction: defaultSchedulerSourceProfileAction,
		ThemeDarkStart:               defaultThemeDarkStart,
		ThemeLightStart:              defaultThemeLightStart,
		ThemeMode:                    defaultThemeMode,
		ProbeNormalizeOptions: probecore.ProbeConfigNormalizeOptions{
			MaxTCPRoutines:    maxDesktopTCPRoutines,
			MaxStage3Routines: maxDesktopStage3Routines,
		},
	}
}
