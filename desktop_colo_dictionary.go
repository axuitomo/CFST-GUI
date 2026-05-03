package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/XIU2/CloudflareSpeedTest/internal/colodict"
)

func desktopColoDictionaryPaths() colodict.Paths {
	return colodict.DefaultPaths(filepath.Dir(desktopConfigFilePath()))
}

func (a *App) LoadColoDictionaryStatus() DesktopCommandResult {
	status, err := colodict.StatusForPaths(desktopColoDictionaryPaths())
	if err != nil {
		return desktopCommandResult("COLO_DICTIONARY_STATUS_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("COLO_DICTIONARY_STATUS_READY", status, "COLO 词典状态已读取。", true, nil, nil)
}

func (a *App) UpdateColoDictionary(payload map[string]any) DesktopCommandResult {
	sourceURL := strings.TrimSpace(stringValue(firstNonNil(payload["source_url"], payload["sourceUrl"]), colodict.DefaultGeofeedURL))
	result, err := colodict.Update(context.Background(), colodict.UpdateOptions{
		Paths:     desktopColoDictionaryPaths(),
		SourceURL: sourceURL,
	})
	if err != nil {
		return desktopCommandResult("COLO_DICTIONARY_UPDATE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("COLO_DICTIONARY_UPDATE_OK", result.Status, "COLO 原始词典已拉取。", true, nil, result.Warnings)
}

func (a *App) ProcessColoDictionary(payload map[string]any) DesktopCommandResult {
	result, err := colodict.Process(colodict.UpdateOptions{
		Paths: desktopColoDictionaryPaths(),
	})
	if err != nil {
		return desktopCommandResult("COLO_DICTIONARY_PROCESS_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("COLO_DICTIONARY_PROCESS_OK", result.Status, "COLO 词典已本地处理。", true, nil, result.Warnings)
}
