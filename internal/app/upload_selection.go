package app

import "github.com/axuitomo/CFST-GUI/internal/appcore"

func BuildUploadSelection(snapshot map[string]any, rows []ProbeRow, metric string) (UploadSelectionResult, error) {
	return appcore.BuildUploadSelection(snapshot, rows, metric)
}

func filterRowsForCloudflareRecordType(rows []ProbeRow, recordType string) []ProbeRow {
	return appcore.FilterRowsForCloudflareRecordType(rows, recordType)
}
