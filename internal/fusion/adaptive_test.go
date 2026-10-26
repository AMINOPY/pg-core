package fusion

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

type adaptiveTestDialer struct{}

func (adaptiveTestDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("unused")
}

func adaptivePath(id string, healthy bool) AdaptivePath {
	return AdaptivePath{
		Route: Route{Path: Path{ID: id, Healthy: healthy, Weight: 1}, Dialer: adaptiveTestDialer{}},
		Quality: QualityReport{Samples: 5, Successes: 5, SuccessRate: 1, PacketLoss: 0, MeanRTT: 40 * time.Millisecond},
	}
}

func TestDecideAdaptivePathsChoosesBackupWhenPrimaryBlocked(t *testing.T) {
	paths := []AdaptivePath{adaptivePath("main", true), adaptivePath("backup", true)}
	got, err := DecideAdaptivePaths("main", paths, AdaptiveThresholds{
		MinSuccessRate: 0.8, MaxPacketLoss: 0.2, MaxMeanRTT: time.Second,
	}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != AdaptiveBackup || got.BackupID != "backup" {
		t.Fatalf("unexpected decision: %+v", got)
	}
}

func TestDecideAdaptivePathsEnablesDualFlowOnlyWhenBothUsable(t *testing.T) {
	paths := []AdaptivePath{adaptivePath("main", true), adaptivePath("backup", true)}
	got, err := DecideAdaptivePaths("main", paths, AdaptiveThresholds{MinSuccessRate: 0.8, MaxPacketLoss: 0.2}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != AdaptiveDualFlow {
		t.Fatalf("mode = %q, want dual-flow", got.Mode)
	}

	paths[1] = adaptivePath("backup", false)
	got, err = DecideAdaptivePaths("main", paths, AdaptiveThresholds{MinSuccessRate: 0.8, MaxPacketLoss: 0.2}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != AdaptivePrimary {
		t.Fatalf("mode = %q, want primary", got.Mode)
	}
}

func TestDecideAdaptivePathsOfflineWhenNoUsableBackup(t *testing.T) {
	paths := []AdaptivePath{adaptivePath("main", false), adaptivePath("backup", false)}
	got, err := DecideAdaptivePaths("main", paths, AdaptiveThresholds{MinSuccessRate: 0.8, MaxPacketLoss: 0.2}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != AdaptiveOffline {
		t.Fatalf("mode = %q, want offline", got.Mode)
	}
}

func TestDecideAdaptivePathsRejectsInvalidThresholdsAndReports(t *testing.T) {
	paths := []AdaptivePath{adaptivePath("main", true)}
	if _, err := DecideAdaptivePaths("main", paths, AdaptiveThresholds{MinSuccessRate: 2}, false, false); err == nil {
		t.Fatal("expected invalid threshold error")
	}
	paths[0].Quality.SuccessRate = 2
	if _, err := DecideAdaptivePaths("main", paths, AdaptiveThresholds{}, false, false); err == nil {
		t.Fatal("expected invalid quality report error")
	}
}
