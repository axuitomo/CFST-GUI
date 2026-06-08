package app

import "github.com/axuitomo/CFST-GUI/internal/appcore"

func BuildUploadSelection(snapshot map[string]any, rows []ProbeRow, metric string) (UploadSelectionResult, error) {
	return appcore.BuildUploadSelectionWithColoPaths(snapshot, rows, metric, desktopColoDictionaryPaths())
}

func filterRowsForCloudflareRecordType(rows []ProbeRow, recordType string) []ProbeRow {
	return appcore.FilterRowsForCloudflareRecordType(rows, recordType)
}
