package appcore

func (s *Service) invokeStorageMaintenance(command, payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	switch command {
	case "storage.cleanup":
		policy := CleanupPolicy{TaskArchiveRetentionDays: intValue(firstNonNil(payload["task_archive_retention_days"], payload["taskArchiveRetentionDays"]), 30), DiagnosticRetentionDays: intValue(firstNonNil(payload["diagnostic_retention_days"], payload["diagnosticRetentionDays"]), 7)}
		return NewCommandResult("STORAGE_CLEANUP_STARTED", s.runStorageCleanup(policy), "存储清理已完成。", true, nil, nil)
	case "storage.cleanup_status":
		return NewCommandResult("STORAGE_CLEANUP_STATUS_READY", s.storageCleanupStatus(), "存储清理状态已读取。", true, nil, nil)
	case "storage.force_clean_legacy":
		return NewCommandResult("STORAGE_LEGACY_CLEANED", s.forceCleanLegacy(), "旧版存储文件已清理。", true, nil, nil)
	default:
		return NewCommandResult("COMMAND_UNKNOWN", nil, "unknown storage command", false, nil, nil)
	}
}
