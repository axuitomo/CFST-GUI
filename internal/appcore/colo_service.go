package appcore

import (
	"context"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

func (s *Service) invokeColoStatus(payloadJSON string) CommandResult {
	if _, err := decodeCommandObject(payloadJSON); err != nil {
		return NewCommandResult("COLO_DICTIONARY_STATUS_FAILED", nil, err.Error(), false, nil, nil)
	}
	status, err := colodict.StatusForPaths(s.ColoPaths())
	if err != nil {
		return NewCommandResult("COLO_DICTIONARY_STATUS_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("COLO_DICTIONARY_STATUS_READY", status, "COLO dictionary status read.", true, nil, nil)
}

func (s *Service) invokeColoUpdate(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COLO_DICTIONARY_UPDATE_FAILED", nil, err.Error(), false, nil, nil)
	}
	s.mu.RLock()
	client := s.options.HTTPClient
	s.mu.RUnlock()
	result, err := colodict.Update(context.Background(), colodict.UpdateOptions{
		Client:    client,
		Paths:     s.ColoPaths(),
		SourceURL: strings.TrimSpace(stringValue(firstNonNil(payload["source_url"], payload["sourceUrl"]), colodict.DefaultGeofeedURL)),
	})
	if err != nil {
		return NewCommandResult("COLO_DICTIONARY_UPDATE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("COLO_DICTIONARY_UPDATE_OK", result.Status, "COLO source dictionary updated.", true, nil, result.Warnings)
}

func (s *Service) invokeColoProcess(payloadJSON string) CommandResult {
	if _, err := decodeCommandObject(payloadJSON); err != nil {
		return NewCommandResult("COLO_DICTIONARY_PROCESS_FAILED", nil, err.Error(), false, nil, nil)
	}
	result, err := colodict.Process(colodict.UpdateOptions{Paths: s.ColoPaths()})
	if err != nil {
		return NewCommandResult("COLO_DICTIONARY_PROCESS_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("COLO_DICTIONARY_PROCESS_OK", result.Status, "COLO dictionary processed.", true, nil, result.Warnings)
}
