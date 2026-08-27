package appcore

import (
	"context"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/githubcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type GitHubExportConfig = githubcore.Config
type GitHubExportResult = githubcore.ExportResult
type GitHubContentsResponse = githubcore.ContentsResponse
type GitHubPutContentsResponse = githubcore.PutContentsResponse
type GitHubContentsPutRequest = githubcore.ContentsPutRequest
type GitHubExportClient = githubcore.Client

func GitHubExportConfigFromPayload(payload map[string]any, defaults githubcore.ConfigDefaults) (GitHubExportConfig, []string, error) {
	return githubcore.ParseConfigFromPayload(payload, defaults)
}

func GitHubExportConfigFromSnapshot(config map[string]any, defaults githubcore.ConfigDefaults) (GitHubExportConfig, []string, error) {
	return githubcore.ParseConfigFromSnapshot(config, defaults)
}

func GitHubCSVEncodingFromPayload(payload map[string]any) string {
	return githubcore.CSVEncodingFromPayload(payload)
}

func GitHubExportCSVTargetFileName(payload map[string]any, targetValue string, fallback string) string {
	return githubcore.ExportCSVTargetFileName(payload, targetValue, fallback)
}

func ProbeRowsFromAny(value any) []probecore.ProbeRow {
	return githubcore.ProbeRowsFromAny(value)
}

func EncodeProbeRowsForGitHub(rows []probecore.ProbeRow, cfg GitHubExportConfig) ([]byte, int, error) {
	return githubcore.EncodeProbeRowsForGitHub(rows, cfg)
}

func ReadProbeRowsForGitHubFromCSV(path string) ([]probecore.ProbeRow, error) {
	rows, err := ReadProbeResultRowsFromCSV(path)
	if err != nil {
		return nil, err
	}
	return ResultRowsToProbeRows(rows), nil
}

func ExportCSVToGitHub(ctx context.Context, cfg GitHubExportConfig, taskID string, body []byte, rowCount int, now time.Time, client *GitHubExportClient) (GitHubExportResult, error) {
	return githubcore.ExportCSV(ctx, client, cfg, taskID, body, rowCount, now)
}
