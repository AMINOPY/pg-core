package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/AMINOPY/pg-core/internal/config"
	"github.com/AMINOPY/pg-core/internal/diagnostics"
)

func main() {
	configPath := flag.String("config", "", "path to a Chimera JSON configuration file (validation only)")
	diagnose := flag.String("diagnose", "", "run bounded DNS + HTTPS checks for a hostname")
	listEngines := flag.Bool("list-engines", false, "list known engine IDs and implementation status")
	status := flag.Bool("status", false, "show local CLI status (no daemon is currently available)")
	flag.Parse()

	switch {
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
	if cfg.Fronting != nil && cfg.Fronting.Enabled() {
		fmt.Printf("CDN/SNI: sni=%q edge_ips=%d\n", cfg.Fronting.SNI, len(cfg.Fronting.EdgeIPs))
	}
	fmt.Println("Validation only: no network engine or tunnel was started.")
}
