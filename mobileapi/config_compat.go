package mobileapi

import "github.com/axuitomo/CFST-GUI/internal/probecore"

func sanitizeMobileConfigSnapshot(input map[string]any) map[string]any {
	snapshot := probecore.SanitizeConfigSnapshot(input, mobileConfigSnapshotOptions())
	enforceMobileProbeOnlyScheduler(snapshot)
	return snapshot
}

func mobileConfigSnapshotOptions() probecore.ConfigSnapshotOptions {
	return probecore.ConfigSnapshotOptions{
		CloudflareTTL:                defaultCloudflareTTL,
		DefaultSourceIPLimit:         defaultMobileSourceIPLimit,
		GitHubBranch:                 defaultMobileGitHubExportBranch,
		GitHubCommitMessageTemplate:  defaultMobileGitHubExportCommitMessageTemplate,
		GitHubOwner:                  defaultMobileGitHubExportOwner,
		GitHubPathTemplate:           defaultMobileGitHubExportPathTemplate,
		GitHubRepo:                   defaultMobileGitHubExportRepo,
		IncludePortPolicy:            true,
		IncludeSchedulerRunMetadata:  false,
		IncludeTheme:                 true,
		PortPolicy:                   probecore.PortPolicySourceOverrideGlobal,
		SchedulerConfigSource:        probecore.DefaultSchedulerConfigSource,
		SchedulerSourceProfileAction: probecore.DefaultSchedulerSourceProfileAction,
		ThemeDarkStart:               probecore.DefaultThemeDarkStart,
		ThemeLightStart:              probecore.DefaultThemeLightStart,
		ThemeMode:                    probecore.DefaultThemeMode,
		ProbeNormalizeOptions: probecore.ProbeConfigNormalizeOptions{
			MaxTCPRoutines:    maxMobileTCPRoutines,
			MaxStage3Routines: maxMobileStage3Routines,
		},
	}
}

func enforceMobileProbeOnlyScheduler(snapshot map[string]any) {
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["run_mode"] = defaultMobileSchedulerRunMode
	snapshot["scheduler"] = scheduler
}
