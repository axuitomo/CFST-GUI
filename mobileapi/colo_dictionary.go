package mobileapi

import (
	"context"
	"strings"

	"github.com/XIU2/CloudflareSpeedTest/internal/colodict"
)

func (s *Service) coloDictionaryPaths() colodict.Paths {
	return colodict.DefaultPaths(s.basePath())
}

func (s *Service) LoadColoDictionaryStatus() string {
	status, err := colodict.StatusForPaths(s.coloDictionaryPaths())
	if err != nil {
		return encodeCommand(commandResultFor("COLO_DICTIONARY_STATUS_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("COLO_DICTIONARY_STATUS_READY", status, "COLO 词典状态已读取。", true, nil, nil))
}

func (s *Service) UpdateColoDictionary(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("COLO_DICTIONARY_UPDATE_FAILED", nil, err.Error(), false, nil, nil))
	}
	sourceURL := strings.TrimSpace(stringValue(firstNonNil(payload["source_url"], payload["sourceUrl"]), colodict.DefaultGeofeedURL))
	result, err := colodict.Update(context.Background(), colodict.UpdateOptions{
		Paths:     s.coloDictionaryPaths(),
		SourceURL: sourceURL,
	})
	if err != nil {
		return encodeCommand(commandResultFor("COLO_DICTIONARY_UPDATE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("COLO_DICTIONARY_UPDATE_OK", result.Status, "COLO 原始词典已拉取。", true, nil, result.Warnings))
}

func (s *Service) ProcessColoDictionary(payloadJSON string) string {
	if _, err := decodeObject(payloadJSON); err != nil {
		return encodeCommand(commandResultFor("COLO_DICTIONARY_PROCESS_FAILED", nil, err.Error(), false, nil, nil))
	}
	result, err := colodict.Process(colodict.UpdateOptions{
		Paths: s.coloDictionaryPaths(),
	})
	if err != nil {
		return encodeCommand(commandResultFor("COLO_DICTIONARY_PROCESS_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("COLO_DICTIONARY_PROCESS_OK", result.Status, "COLO 词典已本地处理。", true, nil, result.Warnings))
}
