package mobileapi

import "github.com/axuitomo/CFST-GUI/internal/probecore"

func mobileConfigSnapshotOptions() probecore.ConfigSnapshotOptions {
	return probecore.ConfigSnapshotOptions{
		CloudflareTTL:                probecore.DefaultCloudflareTTL,
		DefaultSourceIPLimit:         probecore.DefaultConfigSnapshotSourceIPLimit,
		GitHubBranch:                 probecore.DefaultGitHubExportBranch,
		GitHubCommitMessageTemplate:  probecore.DefaultGitHubExportCommitMessage,
		GitHubOwner:                  probecore.DefaultGitHubExportOwner,
		GitHubPathTemplate:           probecore.DefaultGitHubExportPathTemplate,
		GitHubRepo:                   probecore.DefaultGitHubExportRepo,
		IncludePortPolicy:            true,
		IncludeTheme:                 true,
		PortPolicy:                   probecore.PortPolicySourceOverrideGlobal,
		SchedulerConfigSource:        probecore.DefaultSchedulerConfigSource,
		SchedulerSourceProfileAction: probecore.DefaultSchedulerSourceProfileAction,
		ThemeDarkStart:               probecore.DefaultThemeDarkStart,
		ThemeLightStart:              probecore.DefaultThemeLightStart,
		ThemeMode:                    probecore.DefaultThemeMode,
		ProbeNormalizeOptions: probecore.ProbeConfigNormalizeOptions{
			MaxTCPRoutines:    probecore.DefaultMaxProbeTCPRoutines,
			MaxStage3Routines: probecore.DefaultMaxProbeStage3Routines,
		},
	}
}
