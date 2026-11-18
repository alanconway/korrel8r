// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	c, err := Load("testdata/config.json")
	require.NoError(t, err)
	want := Configs{
		{
			Source:  "testdata/config.json",
			Include: []string{"config1.yaml", "config.json", "config2.yaml"},
			Tuning: &Tuning{
				RequestTimeout: Duration(time.Second),
			},
		},
		{
			Source: "testdata/config1.yaml",
			Rules: []Rule{
				{Name: "rule1",
					Start:  ClassSpec{Domain: "foo", Classes: []string{"a", "b", "d", "e"}},
					Goal:   ClassSpec{Domain: "bar", Classes: []string{"z"}},
					Result: ResultSpec{Query: "what?"},
				},
			},
		},
		{
			Source: "testdata/config2.yaml",
			Rules: []Rule{
				{Name: "rule2",
					Start:  ClassSpec{Domain: "foo", Classes: []string{"d", "e"}},
					Goal:   ClassSpec{Domain: "bar", Classes: []string{"q"}},
					Result: ResultSpec{Query: "blah"}}},
		},
	}
	assert.Equal(t, want, c)
}

func TestLoad_bad_tuning(t *testing.T) {
	_, err := Load("testdata/bad-tuning.json")
	require.EqualError(t, err, "unexpected tuning section in included configuration: testdata/config.json")
}

func TestLoadMCP(t *testing.T) {
	dir := t.TempDir()
	top := filepath.Join(dir, "top.yaml")
	included := filepath.Join(dir, "included.yaml")
	require.NoError(t, os.WriteFile(top, []byte("mcp:\n  console: off\ninclude:\n  - included.yaml\n"), 0600))
	require.NoError(t, os.WriteFile(included, []byte("{}\n"), 0600))
	configs, err := Load(top)
	require.NoError(t, err)
	require.Equal(t, "false", configs[0].MCP.Console) // YAML 1.1 treats unquoted off as false.

	require.NoError(t, os.WriteFile(included, []byte("mcp:\n  console: on\n"), 0600))
	_, err = Load(top)
	require.ErrorContains(t, err, "unexpected mcp section in included configuration")
}

func TestConfigs_Expand(t *testing.T) {
	c := Configs{
		{
			Aliases: []Class{
				{Name: "x", Domain: "foo", Classes: []string{"p", "q"}},
				{Name: "y", Domain: "foo", Classes: []string{"x", "a"}},
				{Name: "z", Domain: "foo", Classes: []string{"u", "v"}},
			},
			Rules: []Rule{
				{
					Name:   "r",
					Start:  ClassSpec{Domain: "foo", Classes: []string{"y"}},
					Goal:   ClassSpec{Domain: "foo", Classes: []string{"z"}},
					Result: ResultSpec{Query: "dummy"},
				},
			},
		},
	}
	require.NoError(t, expand(c))
	want := Configs{
		{
			Rules: []Rule{
				{
					Name:   "r",
					Start:  ClassSpec{Domain: "foo", Classes: []string{"p", "q", "a"}},
					Goal:   ClassSpec{Domain: "foo", Classes: []string{"u", "v"}},
					Result: ResultSpec{Query: "dummy"},
				},
			},
		},
	}
	assert.Equal(t, want, c)
}

func TestLoad_statusRules(t *testing.T) {
	c, err := Load("testdata/status-rules.yaml")
	require.NoError(t, err)
	want := Configs{
		{
			Source: "testdata/status-rules.yaml",
			StatusRules: []StatusRule{
				{
					Name:   "HasFinalizer",
					Start:  ClassSpec{Domain: "k8s"},
					Status: "{{- with .metadata.finalizers}}has-finalizer{{end}}",
				},
				{
					Name:   "PodPhase",
					Start:  ClassSpec{Domain: "k8s", Classes: []string{"Pod"}},
					Status: "phase-{{.status.phase}}",
				},
			},
		},
	}
	assert.Equal(t, want, c)
}

func TestConfigs_Expand_sameGroupDifferentDomain(t *testing.T) {
	c := Configs{
		{
			Aliases: []Class{
				{Name: "x", Domain: "foo", Classes: []string{"p", "q"}},
				{Name: "x", Domain: "bar", Classes: []string{"bbq"}},
			},
			Rules: []Rule{
				{
					Name:   "r1",
					Start:  ClassSpec{Domain: "foo", Classes: []string{"a", "x"}},
					Goal:   ClassSpec{Domain: "bar", Classes: []string{"x"}},
					Result: ResultSpec{Query: "help"},
				},
			},
		},
	}
	require.NoError(t, expand(c))
	want := Configs{
		{
			Rules: []Rule{{
				Name:   "r1",
				Start:  ClassSpec{Domain: "foo", Classes: []string{"a", "p", "q"}},
				Goal:   ClassSpec{Domain: "bar", Classes: []string{"bbq"}},
				Result: ResultSpec{Query: "help"}}},
		}}
	assert.Equal(t, want, c)
}
