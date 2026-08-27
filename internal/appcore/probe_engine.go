package appcore

import (
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

type ProbeEngineOptions struct {
	CaptureAddress   string
	ColoPaths        colodict.Paths
	OutputFile       string
	SourceColoFilter task.SourceColoFilterMap
	Hooks            task.Hooks
}

func NewProbeEngine(cfg probecore.ProbeConfig, options ProbeEngineOptions) (*task.Engine, error) {
	resolvedHTTPingColos, err := ResolveConfiguredColos(options.ColoPaths, cfg.HttpingCFColo, "第二阶段全局 COLO 筛选")
	if err != nil {
		return nil, err
	}
	return task.NewEngine(task.Config{
		Routines:               cfg.Routines,
		HeadRoutines:           cfg.HeadRoutines,
		HeadTestCount:          cfg.HeadTestCount,
		HeadMaxDelay:           time.Duration(cfg.HeadMaxDelayMS) * time.Millisecond,
		HeadTimeout:            time.Duration(cfg.Stage2TimeoutMS) * time.Millisecond,
		PingTimes:              cfg.PingTimes,
		SkipFirstLatencySample: cfg.SkipFirstLatency,
		TCPConnectTimeout:      time.Duration(cfg.Stage1TimeoutMS) * time.Millisecond,
		TestCount:              cfg.TestCount,
		DownloadRoutines:       cfg.Stage3Concurrency,
		DownloadGetConcurrency: cfg.DownloadGetConcurrency,
		DownloadBufferKB:       cfg.DownloadBufferKB,
		DownloadHTTPProtocol:   cfg.DownloadHTTPProtocol,
		DownloadSampleInterval: time.Duration(cfg.DownloadSpeedSampleIntervalMS) * time.Millisecond,
		Timeout:                time.Duration(cfg.DownloadTimeSeconds) * time.Second,
		DownloadWarmupDuration: time.Duration(cfg.DownloadWarmupSeconds) * time.Second,
		TCPPort:                cfg.TCPPort,
		URL:                    cfg.URL,
		TraceURL:               cfg.TraceURL,
		TraceColoMode:          cfg.TraceColoMode,
		ColoDictionaryPath:     options.ColoPaths.Colo,
		SourceColoFilters:      options.SourceColoFilter,
		UserAgent:              cfg.UserAgent,
		HostHeader:             cfg.HostHeader,
		SNI:                    cfg.SNI,
		DownloadHostHeader:     cfg.DownloadHostHeader,
		DownloadSNI:            cfg.DownloadSNI,
		RequestHeaders:         cfg.RequestHeaders,
		CaptureAddress:         options.CaptureAddress,
		InsecureSkipVerify:     !cfg.VerifyTLSCertificate,
		Httping:                cfg.Httping,
		HttpingStatusCode:      cfg.HttpingStatusCode,
		HttpingCFColo:          cfg.HttpingCFColo,
		HttpingCFColoMode:      cfg.HttpingCFColoMode,
		HttpingCFColos:         resolvedHTTPingColos,
		MinSpeed:               cfg.MinSpeedMB,
		MinSpeedMetric:         cfg.DownloadSpeedMetric,
		Disable:                cfg.DisableDownload,
		TestAll:                cfg.TestAll,
		RetryMaxAttempts:       cfg.RetryMaxAttempts,
		RetryBackoff:           time.Duration(cfg.RetryBackoffMS) * time.Millisecond,
		CooldownFailures:       cfg.CooldownFailures,
		CooldownDuration:       time.Duration(cfg.CooldownMS) * time.Millisecond,
		IPFile:                 cfg.IPFile,
		IPText:                 cfg.IPText,
		InputMaxDelay:          time.Duration(cfg.MaxDelayMS) * time.Millisecond,
		InputMinDelay:          time.Duration(cfg.MinDelayMS) * time.Millisecond,
		InputMaxLossRate:       float32(cfg.MaxLossRate),
		PrintNum:               cfg.PrintNum,
		Output:                 options.OutputFile,
		OutputAppend:           cfg.ExportAppend,
		OutputCSVEncoding:      cfg.CSVEncoding,
		Debug:                  cfg.Debug,
	}, options.Hooks), nil
}
