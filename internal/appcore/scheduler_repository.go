package appcore

import (
	"encoding/json"
	"errors"
	"os"
)

func (s *Service) LoadSchedulerStatus() (SchedulerStatus, bool, error) {
	path := s.StorageLayout().SchedulerPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SchedulerStatus{}, false, nil
		}
		return SchedulerStatus{}, false, err
	}
	var status SchedulerStatus
	if _, err := UnmarshalJSONCompat(raw, &status); err != nil {
		return SchedulerStatus{}, true, err
	}
	return status, true, nil
}

func (s *Service) SaveSchedulerStatus(status SchedulerStatus) error {
	raw, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.StorageLayout().SchedulerPath(), raw, 0o600)
}
