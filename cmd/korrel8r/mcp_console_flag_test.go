// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package main

import (
	"errors"
	"testing"

	"github.com/korrel8r/korrel8r/pkg/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

type fakeDiscovery struct {
	discovery.DiscoveryInterface
	groups *metav1.APIGroupList
	err    error
}

func (f fakeDiscovery) ServerGroups() (*metav1.APIGroupList, error) { return f.groups, f.err }

func TestOpenShiftDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		groups []metav1.APIGroup
		err    error
		want   bool
	}{
		{"openshift", []metav1.APIGroup{{Name: "route.openshift.io"}}, nil, true},
		{"kubernetes", []metav1.APIGroup{{Name: "apps"}}, nil, false},
		{"unavailable", nil, errors.New("no discovery"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := isOpenShift(fakeDiscovery{groups: &metav1.APIGroupList{Groups: tc.groups}, err: tc.err})
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestConsoleSetting(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured string // mcp.console configuration value, "" for absent.
		flag       string // --mcp-console value, "" for unset.
		want       consoleMode
		wantErr    string
	}{
		{name: "default", want: consoleAuto},
		{name: "config on", configured: "on", want: consoleOn},
		{name: "config true", configured: "true", want: consoleOn},
		{name: "config false", configured: "false", want: consoleOff},
		{name: "config auto", configured: "auto", want: consoleAuto},
		{name: "flag overrides config", configured: "off", flag: "on", want: consoleOn},
		{name: "flag off overrides config", configured: "on", flag: "off", want: consoleOff},
		{name: "flag auto overrides config", configured: "on", flag: "auto", want: consoleAuto},
		{name: "bad config", configured: "bad", wantErr: `mcp.console: invalid value "bad"`},
		{name: "bad config with flag", configured: "bad", flag: "off", wantErr: `mcp.console: invalid value "bad"`},
		{name: "bad flag", flag: "bad", wantErr: `--mcp-console: invalid value "bad"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			addConsoleFlag(cmd)
			if tc.flag != "" {
				require.NoError(t, cmd.Flags().Set(consoleFlagName, tc.flag))
			}
			configs := config.Configs{{}}
			if tc.configured != "" {
				configs[0].MCP = &config.MCP{Console: tc.configured}
			}
			got, err := consoleSetting(cmd, configs)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestConsoleModeEnabled(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mode       consoleMode
		detect     bool
		want       bool
		wantDetect bool // detect must be called.
	}{
		{name: "on", mode: consoleOn, want: true},
		{name: "off", mode: consoleOff, want: false},
		{name: "auto openshift", mode: consoleAuto, detect: true, want: true, wantDetect: true},
		{name: "auto kubernetes", mode: consoleAuto, detect: false, want: false, wantDetect: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			detect := func() bool { called = true; return tc.detect }
			assert.Equal(t, tc.want, tc.mode.enabled(detect))
			assert.Equal(t, tc.wantDetect, called, "detect called")
		})
	}
}
