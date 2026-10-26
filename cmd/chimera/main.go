package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/AMINOPY/pg-core/internal/ai"
	"github.com/AMINOPY/pg-core/internal/config"
	"github.com/AMINOPY/pg-core/internal/diagnostics"
	"github.com/AMINOPY/pg-core/internal/engine/native"
	"github.com/AMINOPY/pg-core/internal/fronting"
	"github.com/AMINOPY/pg-core/internal/quickconnect"
)

func main() {
	configPath := flag.String("config", "", "path to a Chimera JSON configuration file (validation only)")
	run := flag.Bool("run", false, "run the selected native or Xray engine with a local SOCKS5 profile using --config and --profile")
	profilePath := flag.String("profile", "", "path to a native engine profile (JSON), used with --run")
	diagnose := flag.String("diagnose", "", "run bounded DNS + HTTPS checks for a hostname")
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
		fmt.Fprintln(os.Stderr, "--run needs active_engine \"native\"")
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
	rep, _ := a.Health(ctx)
	fmt.Printf("edge check: %s %s\n", rep.Status, rep.ErrorCode)
	<-ctx.Done()
	_ = a.Stop(context.Background())
}
