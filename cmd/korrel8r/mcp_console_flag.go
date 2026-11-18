// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package main

import (
	"fmt"
	"time"

	"github.com/korrel8r/korrel8r/pkg/config"
	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery"
)

const (
	consoleFlagName = "mcp-console"
	consoleFlagHelp = "MCP console tools: auto (OpenShift discovery), on, or off"
)

// addConsoleFlag adds the --mcp-console flag to a command that can run an MCP server.
func addConsoleFlag(cmd *cobra.Command) {
	cmd.Flags().String(consoleFlagName, "auto", consoleFlagHelp)
}

// consoleMode is the configured state of the MCP console tools.
type consoleMode int

const (
	consoleAuto consoleMode = iota // Enable if the cluster is OpenShift.
	consoleOn                      // Always enable.
	consoleOff                     // Always disable.
)

func parseConsoleMode(value string) (consoleMode, error) {
	// Note: YAML 1.1 converts unquoted on/off to true/false when decoding into a string.
	switch value {
	case "auto":
		return consoleAuto, nil
	case "on", "true":
		return consoleOn, nil
	case "off", "false":
		return consoleOff, nil
	default:
		return consoleAuto, fmt.Errorf("invalid value %q: expected auto, on, or off", value)
	}
}

// consoleSetting returns the console mode from the top-level configuration,
// overridden by the --mcp-console flag. Both values are validated.
func consoleSetting(cmd *cobra.Command, configs config.Configs) (consoleMode, error) {
	mode := consoleAuto
	if len(configs) > 0 && configs[0].MCP != nil && configs[0].MCP.Console != "" {
		m, err := parseConsoleMode(configs[0].MCP.Console)
		if err != nil {
			return consoleAuto, fmt.Errorf("mcp.console: %w", err)
		}
		mode = m
	}
	if cmd.Flags().Changed(consoleFlagName) {
		value, err := cmd.Flags().GetString(consoleFlagName)
		if err != nil {
			return consoleAuto, err
		}
		m, err := parseConsoleMode(value)
		if err != nil {
			return consoleAuto, fmt.Errorf("--%v: %w", consoleFlagName, err)
		}
		mode = m
	}
	return mode, nil
}

// enabled resolves the mode to a flag, calling detect only in auto mode.
func (m consoleMode) enabled(detect func() bool) bool {
	switch m {
	case consoleOn:
		return true
	case consoleOff:
		return false
	default:
		enabled := detect()
		if !enabled {
			log.V(1).Info("MCP console tools disabled: not an OpenShift cluster")
		}
		return enabled
	}
}

// detectOpenShift reports whether the default cluster is an OpenShift cluster.
func detectOpenShift() bool {
	cfg, err := k8s.GetConfig()
	if err != nil {
		log.V(1).Info("MCP console tools: cluster configuration unavailable", "error", err)
		return false
	}
	cfg.Timeout = 5 * time.Second
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.V(1).Info("MCP console tools: cluster discovery unavailable", "error", err)
		return false
	}
	return isOpenShift(dc)
}

// isOpenShift reports whether discovery finds OpenShift-only API groups.
func isOpenShift(dc discovery.DiscoveryInterface) bool {
	groups, err := dc.ServerGroups()
	if err != nil {
		log.V(1).Info("MCP console tools: cluster discovery failed", "error", err)
		return false
	}
	for _, group := range groups.Groups {
		if group.Name == "route.openshift.io" {
			return true
		}
	}
	return false
}
