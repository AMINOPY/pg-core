package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/AMINOPY/pg-core/internal/ai"
	"github.com/AMINOPY/pg-core/internal/config"
	"github.com/AMINOPY/pg-core/internal/diagnostics"
	"github.com/AMINOPY/pg-core/internal/engine/native"
	"github.com/AMINOPY/pg-core/internal/engine/xray"
	"github.com/AMINOPY/pg-core/internal/fronting"
	"github.com/AMINOPY/pg-core/internal/ghost"
	"github.com/AMINOPY/pg-core/internal/profileimport"
	"github.com/AMINOPY/pg-core/internal/quickconnect"
)

func main() {
	configPath := flag.String("config", "", "path to a Chimera JSON configuration file (validation and --run)")
	run := flag.Bool("run", false, "run the selected native or Xray engine (local SOCKS5) using --config and --profile")
	profilePath := flag.String("profile", "", "path to a native or Xray engine profile (JSON), used with --run")
	diagnose := flag.String("diagnose", "", "run bounded DNS + HTTPS checks for a hostname")
	ghostHost := flag.String("ghost", "", "run Ghost network symptom analysis for a hostname (not proof of DPI)")
	aiScan := flag.String("ai-scan", "", "run bounded, explainable network diagnostics for a hostname")
	samples := flag.Int("samples", 5, "number of diagnostic samples (1-20)")
	downloadURL := flag.String("download-url", "", "optional trusted HTTPS download-test endpoint (max 10 MiB by default)")
	uploadURL := flag.String("upload-url", "", "optional trusted HTTPS upload-test endpoint (1 MiB by default)")
	listEngines := flag.Bool("list-engines", false, "list known engine IDs and implementation status")
	status := flag.Bool("status", false, "show local CLI status (no daemon is currently available)")
	quickConnect := flag.Bool("quick-connect", false, "start the two-field local CDN/SNI connect UI (no config file required)")
	quickConnectListen := flag.String("quick-connect-listen", "127.0.0.1:8787", "loopback address for the quick-connect UI")
	flag.Parse()

	switch {
	case *ghostHost != "":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		runner := ghost.Runner{
			Checker: diagnostics.Checker{Timeout: 5 * time.Second, Workers: 1},
			Samples: *samples,
			Interval: 200 * time.Millisecond,
		}
		report, err := runner.Run(ctx, *ghostHost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ghost analysis failed: %v\\n", err)
			os.Exit(2)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "encode Ghost report: %v\\n", err)
			os.Exit(1)
		}
		if report.HTTPSuccesses == 0 {
			os.Exit(1)
		}
		return
	case *quickConnect:
		server, err := quickconnect.NewFromEnv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		fmt.Printf("Chimera Quick Connect UI: http://%s\n", *quickConnectListen)
		fmt.Println("Press Ctrl+C to stop the UI and tunnel.")
		if err := quickconnect.Serve(ctx, *quickConnectListen, server); err != nil {
			fmt.Fprintf(os.Stderr, "quick-connect: %v\n", err)
			os.Exit(1)
		}
		return
	case *aiScan != "":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		runner := ai.Runner{
			Checker:     diagnostics.Checker{Timeout: 5 * time.Second, Workers: 1},
			Samples:     *samples,
			Interval:    150 * time.Millisecond,
			DownloadURL: *downloadURL,
			UploadURL:   *uploadURL,
			SpeedTester: ai.SpeedTester{Timeout: 15 * time.Second},
		}
		report, err := runner.Run(ctx, *aiScan)
		if err != nil {
			fmt.Fprintf(os.Stderr, "network diagnostics failed: %v\n", err)
			os.Exit(2)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "encode diagnostics report: %v\n", err)
			os.Exit(1)
		}
		if report.Successful == 0 {
			os.Exit(1)
		}
		if report.SpeedTest != nil && (report.SpeedTest.DownloadError != "" || report.SpeedTest.UploadError != "") {
			os.Exit(1)
		}
		return
	case *diagnose != "":
		checker := diagnostics.Checker{Timeout: 5 * time.Second, Workers: 1}
		result := checker.Check(context.Background(), *diagnose)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "encode diagnostics: %v\n", err)
			os.Exit(1)
		}
		if !result.DNSOK || !result.HTTPSOK {
			os.Exit(1)
		}
		return
	case *listEngines:
		fmt.Println("Known engine IDs (sing-box adapter package exists; CLI does not launch engines):")
		fmt.Println("- sing-box")
		fmt.Println("- xray")
		fmt.Println("- mihomo")
		fmt.Println("- aether")
		return
	case *status:
		fmt.Println("Chimera Core status: CLI available; no background daemon or active tunnel.")
		return
	case *configPath == "":
		fmt.Println("Chimera Core — early scaffold")
		fmt.Println("No network engine is started by this command.")
		fmt.Println("Use --config config.example.json to validate a configuration.")
		fmt.Println("Use --diagnose example.com for a bounded DNS/HTTPS check.")
		fmt.Println("Use --ai-scan example.com for bounded, explainable network diagnostics.")
		fmt.Println("Use --ghost example.com for cautious network symptom analysis (not definitive DPI detection).")
		fmt.Println("Add --download-url and/or --upload-url to opt in to bounded throughput measurements.")
		fmt.Println("Use --quick-connect for the two-field CDN/SNI UI (set CHIMERA_TUNNEL_KEY first).")
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "configuration invalid: %v\n", err)
		os.Exit(2)
	}

	if *run {
		runNative(cfg, *profilePath)
		return
	}

	fmt.Printf("Configuration valid: schema_version=%d profile=%q engine=%q\n", cfg.SchemaVersion, cfg.Profile, cfg.ActiveEngine)
	if cfg.Fronting != nil && cfg.Fronting.Enabled() {
		ips, skipped := fronting.ParseIPs(cfg.Fronting.EdgeIPs)
		fmt.Printf("CDN/SNI: sni=%q edge_ips=%d skipped=%d\n", cfg.Fronting.SNI, len(ips), len(skipped))
	}
	fmt.Println("Validation only: no network engine or tunnel was started.")
}

func runNative(cfg config.Config, profilePath string) {
	if cfg.ActiveEngine == config.EngineXray {
		runXray(cfg, profilePath)
		return
	}
	if cfg.ActiveEngine != config.EngineNative {
		fmt.Fprintln(os.Stderr, "--run supports active_engine \"native\" or \"xray\"")
		os.Exit(2)
	}
	if profilePath == "" {
		fmt.Fprintln(os.Stderr, "--run needs --profile")
		os.Exit(2)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read profile: %v\n", err)
		os.Exit(2)
	}
	a := native.New()
	a.Fronting = cfg.Fronting
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := a.Start(ctx, profile); err != nil {
		fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("SOCKS5 proxy listening on %s (Ctrl+C to stop)\n", a.Addr())
	rep, err := a.Health(ctx)
	if err != nil {
		_ = a.Stop(context.Background())
		fmt.Fprintf(os.Stderr, "native engine health check: %v\n", err)
		return
	}
	fmt.Printf("edge check: %s %s\n", rep.Status, rep.ErrorCode)
	<-ctx.Done()
	if err := a.Stop(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "native engine cleanup warning: %v\n", err)
	}
}

// runXray starts the installed Xray executable with the core's validated CDN/SNI
// settings applied by the adapter. The profile must include a loopback SOCKS5
// inbound; this process does not configure system TUN/routes/DNS.
func runXray(cfg config.Config, profilePath string) {
	if profilePath == "" {
		fmt.Fprintln(os.Stderr, "--run with active_engine \"xray\" needs --profile")
		os.Exit(2)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read Xray profile: %v\n", err)
		os.Exit(2)
	}
	profile, err = profileimport.ToXrayJSON(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import Xray/NPV profile: %v\n", err)
		os.Exit(2)
	}
	proxyAddr, err := xray.LocalSOCKSAddress(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Xray profile must expose a loopback no-auth SOCKS5 inbound: %v\n", err)
		os.Exit(2)
	}
	binary := strings.TrimSpace(os.Getenv("CHIMERA_XRAY_BINARY"))
	if binary == "" {
		binary = "xray"
	}
	adapter := xray.New(binary)
	adapter.Fronting = cfg.Fronting
	adapter.HealthTarget = strings.TrimSpace(os.Getenv("CHIMERA_XRAY_HEALTH_TARGET"))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := adapter.Start(ctx, profile); err != nil {
		fmt.Fprintf(os.Stderr, "start Xray: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := adapter.Stop(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "stop Xray cleanup warning: %v\n", err)
		}
	}()
	fmt.Printf("Xray local SOCKS5 proxy listening on %s (Ctrl+C to stop)\n", proxyAddr)
	if cfg.Fronting != nil && cfg.Fronting.Enabled() {
		fmt.Printf("CDN/SNI applied: SNI=%s edge-IP-count=%d\n", cfg.Fronting.SNI, len(cfg.Fronting.EdgeIPs))
	}
	report, err := adapter.Health(ctx)
	if err != nil {
		_ = adapter.Stop(context.Background())
		fmt.Fprintf(os.Stderr, "Xray health check: %v\n", err)
		return
	}
	fmt.Printf("Xray health: %s %s\n", report.Status, report.ErrorCode)
	if report.Status == "degraded" {
		fmt.Fprintln(os.Stderr, "Warning: set CHIMERA_XRAY_HEALTH_TARGET=host:port to verify TCP connectivity through SOCKS; process liveness alone is not tunnel health.")
	}
	<-ctx.Done()
}
