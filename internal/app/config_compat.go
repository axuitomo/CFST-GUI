package app

import "github.com/axuitomo/CFST-GUI/internal/probecore"

func desktopConfigSnapshotOptions() probecore.ConfigSnapshotOptions {
	return probecore.ConfigSnapshotOptions{
		CloudflareTTL:                probecore.DefaultCloudflareTTL,
		DefaultSourceIPLimit:         probecore.DefaultConfigSnapshotSourceIPLimit,
		GitHubBranch:                 probecore.DefaultGitHubExportBranch,
		GitHubCommitMessageTemplate:  probecore.DefaultGitHubExportCommitMessage,
		GitHubOwner:                  probecore.DefaultGitHubExportOwner,
		GitHubPathTemplate:           probecore.DefaultGitHubExportPathTemplate,
		GitHubRepo:                   probecore.DefaultGitHubExportRepo,
		IncludePortPolicy:            true,
		IncludeSchedulerMetadata:     true,
		IncludeTheme:                 true,
		PortPolicy:                   defaultPortPolicy,
		SchedulerConfigSource:        defaultSchedulerConfigSource,
		SchedulerSourceProfileAction: defaultSchedulerSourceProfileAction,
		ThemeDarkStart:               defaultThemeDarkStart,
		ThemeLightStart:              defaultThemeLightStart,
		ThemeMode:                    defaultThemeMode,
		ProbeNormalizeOptions: probecore.ProbeConfigNormalizeOptions{
			MaxTCPRoutines:    probecore.DefaultMaxProbeTCPRoutines,
			MaxStage3Routines: probecore.DefaultMaxProbeStage3Routines,
		},
	}
}
