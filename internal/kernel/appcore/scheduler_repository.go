package appcore

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/fileio"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/jsoncompat"
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
	if _, err := jsoncompat.UnmarshalJSONCompat(raw, &status); err != nil {
		return SchedulerStatus{}, true, err
	}
	return status, true, nil
}

func (s *Service) SaveSchedulerStatus(status SchedulerStatus) error {
	raw, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return fileio.WriteFileAtomic(s.StorageLayout().SchedulerPath(), raw, 0o600)
}
