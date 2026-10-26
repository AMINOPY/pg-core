package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/AMINOPY/pg-core/internal/config"
	"github.com/AMINOPY/pg-core/internal/engine/xray"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

// runXray starts an installed Xray executable with a supplied profile. The
// shared config's fronting settings are applied by the adapter before Xray
// validates or starts the private, temporary profile.
func runXray(cfg config.Config, profilePath string) {
	if profilePath == "" {
		fmt.Fprintln(os.Stderr, "--run with active_engine \"xray\" needs --profile <xray-config.json>")
		os.Exit(2)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read Xray profile: %v\n", err)
		os.Exit(2)
	}
	proxyAddr, err := localSOCKSAddress(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Xray profile must expose a loopback-only, no-auth SOCKS inbound: %v\n", err)
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
	fmt.Printf("Xray local SOCKS5 proxy: %s\n", proxyAddr)
	if cfg.Fronting != nil && cfg.Fronting.Enabled() {
		ips, _ := fronting.ParseIPs(cfg.Fronting.EdgeIPs)
		fmt.Printf("CDN/SNI fronting enabled: SNI=%s edge-IP-count=%d\n", cfg.Fronting.SNI, len(ips))
	}
	report, healthErr := adapter.Health(ctx)
	if healthErr != nil {
		fmt.Fprintf(os.Stderr, "Xray health check: %v\n", healthErr)
	} else {
		fmt.Printf("Xray status: %s (%s)\n", report.Status, report.ErrorCode)
		if report.Status == "failed" {
			fmt.Fprintln(os.Stderr, "Xray process/data-path health check failed")
		}
	}
	fmt.Println("Configure an application to use the loopback SOCKS5 proxy; this command does not install system-wide routes or TUN.")
	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adapter.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "stop Xray: %v\n", err)
		os.Exit(1)
	}
}

func localSOCKSAddress(profile []byte) (string, error) {
	var root struct {
		Inbounds []struct {
			Listen   string `json:"listen"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
			Settings struct {
				Auth     string            `json:"auth"`
				Accounts []json.RawMessage `json:"accounts"`
			} `json:"settings"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(profile, &root); err != nil {
		return "", errors.New("invalid JSON")
	}
	for _, in := range root.Inbounds {
		if !strings.EqualFold(in.Protocol, "socks") || in.Port < 1 || in.Port > 65535 ||
			(in.Settings.Auth != "" && !strings.EqualFold(in.Settings.Auth, "noauth")) ||
			len(in.Settings.Accounts) != 0 {
			continue
		}
		host := strings.TrimSpace(in.Listen)
		if host == "" {
			host = "127.0.0.1"
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			continue
		}
		return net.JoinHostPort(ip.String(), strconv.Itoa(in.Port)), nil
	}
	return "", errors.New("no explicit loopback SOCKS inbound found")
}
