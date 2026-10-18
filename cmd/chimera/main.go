package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/AMINOPY/pg-core/internal/ai"
	"github.com/AMINOPY/pg-core/internal/config"
	"github.com/AMINOPY/pg-core/internal/diagnostics"
)

func main() {
	configPath := flag.String("config", "", "path to a Chimera JSON configuration file (validation only)")
	diagnose := flag.String("diagnose", "", "run one DNS + HTTPS check for a hostname")
	aiScan := flag.String("ai-scan", "", "run bounded network diagnostics for a hostname")
	samples := flag.Int("samples", 5, "number of diagnostic samples (1-20)")
	downloadURL := flag.String("download-url", "", "optional trusted HTTPS endpoint returning download-test bytes (max 10 MiB by default)")
	uploadURL := flag.String("upload-url", "", "optional trusted HTTPS endpoint accepting upload-test POST data (1 MiB by default)")
	listEngines := flag.Bool("list-engines", false, "list known engine IDs and implementation status")
	status := flag.Bool("status", false, "show local CLI status (no daemon is currently available)")
	flag.Parse()

	switch {
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
		fmt.Println("Chimera Core — modular network core")
		fmt.Println("No network engine is started by this command.")
		fmt.Println("Use --config config.example.json to validate a configuration.")
		fmt.Println("Use --diagnose example.com for a single DNS/HTTPS check.")
		fmt.Println("Use --ai-scan example.com for bounded, explainable network diagnostics.")
		fmt.Println("Add --download-url and/or --upload-url to opt in to bounded throughput measurements.")
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

	fmt.Printf("Configuration valid: schema_version=%d profile=%q engine=%q\n", cfg.SchemaVersion, cfg.Profile, cfg.ActiveEngine)
	fmt.Println("Validation only: no network engine or tunnel was started.")
}
