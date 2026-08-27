package engine

import "testing"

func TestDefaultConfigIncludesSplitGuards(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MinSamplesSplit != 8 {
		t.Fatalf("MinSamplesSplit = %d, want 8", cfg.MinSamplesSplit)
	}
	if cfg.MaxTreeNodes != 4096 {
		t.Fatalf("MaxTreeNodes = %d, want 4096", cfg.MaxTreeNodes)
	}
	if cfg.MinSplitInformationGain != 0.10 {
		t.Fatalf("MinSplitInformationGain = %f, want 0.10", cfg.MinSplitInformationGain)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() error = %v", err)
	}

	treeCfg := cfg.ToTreeConfig()
	if treeCfg.MaxNodes != cfg.MaxTreeNodes || treeCfg.MinInformationGain != cfg.MinSplitInformationGain {
		t.Fatalf("ToTreeConfig() split guards = %d/%f, want %d/%f", treeCfg.MaxNodes, treeCfg.MinInformationGain, cfg.MaxTreeNodes, cfg.MinSplitInformationGain)
	}
}

func TestConfigApplyDefaultsIncludesSplitGuards(t *testing.T) {
	var cfg Config
	cfg.ApplyDefaults()
	defaults := DefaultConfig()
	if cfg.MinSamplesSplit != defaults.MinSamplesSplit || cfg.MaxTreeNodes != defaults.MaxTreeNodes || cfg.MinSplitInformationGain != defaults.MinSplitInformationGain {
		t.Fatalf("ApplyDefaults() split guards = %d/%d/%f, want %d/%d/%f", cfg.MinSamplesSplit, cfg.MaxTreeNodes, cfg.MinSplitInformationGain, defaults.MinSamplesSplit, defaults.MaxTreeNodes, defaults.MinSplitInformationGain)
	}
}
