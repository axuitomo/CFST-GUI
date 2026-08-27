package bandit

import (
	"net/netip"
	"testing"
)

func updateNodeSamples(node *ArmNode, count int) {
	for i := 0; i < count; i++ {
		node.Update(true, 20, 3000)
	}
}

func TestArmTreeSplitRequiresMinimumSamples(t *testing.T) {
	prefix := netip.MustParsePrefix("198.51.96.0/20")
	tree := NewArmTree([]netip.Prefix{prefix}, DefaultTreeConfig())
	node := tree.GetNode(prefix)

	updateNodeSamples(node, 7)
	if children := tree.SplitNode(node); children != nil {
		t.Fatalf("SplitNode() with 7 samples created %d children, want none", len(children))
	}

	updateNodeSamples(node, 1)
	children := tree.SplitNode(node)
	if len(children) != 4 {
		t.Fatalf("SplitNode() with 8 samples created %d children, want 4", len(children))
	}
	if tree.Size() != 5 {
		t.Fatalf("tree size = %d, want 5", tree.Size())
	}
}

func TestArmTreeSplitRequiresInformationGain(t *testing.T) {
	cfg := DefaultTreeConfig()
	cfg.MinInformationGain = 1.0
	prefix := netip.MustParsePrefix("198.51.96.0/20")
	tree := NewArmTree([]netip.Prefix{prefix}, cfg)
	node := tree.GetNode(prefix)
	updateNodeSamples(node, cfg.MinSamples)

	if gain := node.InformationGain(); gain >= cfg.MinInformationGain {
		t.Fatalf("test fixture information gain = %f, want below %f", gain, cfg.MinInformationGain)
	}
	if children := tree.SplitNode(node); children != nil {
		t.Fatalf("SplitNode() created %d children below information threshold", len(children))
	}
	if candidates := tree.GetSplitCandidates(10); len(candidates) != 0 {
		t.Fatalf("GetSplitCandidates() returned %d candidates below information threshold", len(candidates))
	}
}

func TestArmTreeSplitRespectsNodeCapacityAtomically(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		maxNodes  int
		wantSplit bool
		wantSize  int
		wantChild int
	}{
		{name: "IPv4 insufficient capacity", prefix: "198.51.96.0/20", maxNodes: 4, wantSize: 1},
		{name: "IPv4 exact capacity", prefix: "198.51.96.0/20", maxNodes: 5, wantSplit: true, wantSize: 5, wantChild: 4},
		{name: "IPv6 insufficient capacity", prefix: "2001:db8:1000::/48", maxNodes: 16, wantSize: 1},
		{name: "IPv6 exact capacity", prefix: "2001:db8:1000::/48", maxNodes: 17, wantSplit: true, wantSize: 17, wantChild: 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultTreeConfig()
			cfg.MaxNodes = tt.maxNodes
			prefix := netip.MustParsePrefix(tt.prefix)
			tree := NewArmTree([]netip.Prefix{prefix}, cfg)
			node := tree.GetNode(prefix)
			updateNodeSamples(node, cfg.MinSamples)

			children := tree.SplitNode(node)
			if got := children != nil; got != tt.wantSplit {
				t.Fatalf("SplitNode() split = %v, want %v", got, tt.wantSplit)
			}
			if len(children) != tt.wantChild {
				t.Fatalf("child count = %d, want %d", len(children), tt.wantChild)
			}
			if tree.Size() != tt.wantSize {
				t.Fatalf("tree size = %d, want %d", tree.Size(), tt.wantSize)
			}
			if node.Stats().IsSplit != tt.wantSplit {
				t.Fatalf("node split state = %v, want %v", node.Stats().IsSplit, tt.wantSplit)
			}
		})
	}
}
