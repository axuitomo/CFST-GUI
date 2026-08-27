package appcore

type StorageSetResult struct {
	Migration any
	Storage   any
}

type StorageHealthResult struct {
	Health  any
	Storage any
}

type StorageCommandHooks struct {
	Health func(map[string]any) (StorageHealthResult, error)
	Set    func(map[string]any) (StorageSetResult, error)
}

func (s *Service) invokeStorage(command, payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	s.mu.RLock()
	hooks := s.options.StorageCommands
	s.mu.RUnlock()

	switch command {
	case "storage.set":
		result := StorageSetResult{
			Migration: map[string]any{"copied": []string{}, "failed": []string{}, "skipped": []string{}},
			Storage:   s.archiveStorageState(),
		}
		if hooks.Set != nil {
			result, err = hooks.Set(payload)
		}
		data := map[string]any{"migration": result.Migration, "storage": result.Storage}
		if err != nil {
			return NewCommandResult("STORAGE_SET_FAILED", data, err.Error(), false, nil, nil)
		}
		return NewCommandResult("STORAGE_SET_DEPRECATED", data, "当前版本不再支持自定义储存目录，已固定使用应用数据目录。", true, nil, nil)
	case "storage.health":
		result := StorageHealthResult{Storage: s.archiveStorageState()}
		if hooks.Health != nil {
			result, err = hooks.Health(payload)
		} else {
			result.Health = s.storageHealth(payload)
		}
		if err != nil {
			return NewCommandResult("STORAGE_HEALTH_FAILED", nil, err.Error(), false, nil, nil)
		}
		return NewCommandResult("STORAGE_HEALTH_READY", map[string]any{
			"health":  result.Health,
			"storage": result.Storage,
		}, "应用数据目录健康检查已完成。", true, nil, nil)
	default:
		return NewCommandResult("COMMAND_UNKNOWN", nil, "unknown storage command", false, nil, nil)
	}
}
