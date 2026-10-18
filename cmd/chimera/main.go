package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/AMINOPY/pg-core/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to a Chimera JSON configuration file")
	flag.Parse()

	if *configPath == "" {
		fmt.Println("Chimera Core — scaffold")
		fmt.Println("No network engine is started by this command.")
		fmt.Println("Use --config config.example.json to validate a configuration.")
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

	fmt.Printf("Configuration valid: profile=%q engine=%q\n", cfg.Profile, cfg.ActiveEngine)
	fmt.Println("Validation only: no network engine or tunnel was started.")
}
