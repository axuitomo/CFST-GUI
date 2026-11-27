package appcore

import (
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/notify"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/scheduler"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/sources"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/storage"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/taskstate"
)

const (
	CommandSchemaVersion    = contracts.CommandSchemaVersion
	EventSchemaVersion      = contracts.EventSchemaVersion
	ProbeEventChannel       = contracts.ProbeEventChannel
	UploadNotificationEvent = contracts.UploadNotificationEvent

	SchedulerSourceProfileActionUpdate = sources.SchedulerSourceProfileActionUpdate
)

type (
	CommandResult       = contracts.CommandResult
	ProbeEvent          = contracts.ProbeEvent
	EventSinkFunc       = contracts.EventSinkFunc
	StorageLayout       = contracts.StorageLayout
	StorageSetResult    = contracts.StorageSetResult
	StorageHealthResult = contracts.StorageHealthResult
	StorageCommandHooks = contracts.StorageCommandHooks
	ConfigPolicy        = contracts.ConfigPolicy

	TaskSnapshot = taskstate.TaskSnapshot

	SchedulerConfig       = scheduler.SchedulerConfig
	SchedulerStatus       = scheduler.SchedulerStatus
	SchedulerTimingConfig = scheduler.SchedulerTimingConfig
)

var (
	NewCommandResult = contracts.NewCommandResult
	NewConfigPolicy  = contracts.NewConfigPolicy

	RunStorageMigration = storage.RunStorageMigration

	TaskSnapshotFromEvent = taskstate.TaskSnapshotFromEvent
	MergeTaskSnapshots    = taskstate.MergeTaskSnapshots

	SchedulerStageActive = scheduler.SchedulerStageActive
	NextSchedulerRun     = scheduler.NextSchedulerRun

	BuildUploadSelection = notify.BuildUploadSelection
)
