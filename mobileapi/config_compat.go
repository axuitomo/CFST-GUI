package mobileapi

import "github.com/axuitomo/CFST-GUI/internal/probecore"

func sanitizeMobileConfigSnapshot(input map[string]any) map[string]any {
	return probecore.SanitizeConfigSnapshot(input, mobileConfigSnapshotOptions())
}

func mobileConfigSnapshotOptions() probecore.ConfigSnapshotOptions {
	return probecore.ConfigSnapshotOptions{
		CloudflareTTL:               defaultCloudflareTTL,
		DefaultSourceIPLimit:        defaultMobileSourceIPLimit,
		GitHubBranch:                defaultMobileGitHubExportBranch,
		GitHubCommitMessageTemplate: defaultMobileGitHubExportCommitMessageTemplate,
		GitHubOwner:                 defaultMobileGitHubExportOwner,
		GitHubPathTemplate:          defaultMobileGitHubExportPathTemplate,
		GitHubRepo:                  defaultMobileGitHubExportRepo,
		ProbeNormalizeOptions: probecore.ProbeConfigNormalizeOptions{
			MaxTCPRoutines:    maxMobileTCPRoutines,
			MaxStage3Routines: maxMobileStage3Routines,
		},
	}
}
