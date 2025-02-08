package helm

import (
	"testing"

	"github.com/kimerize/kimerize/lib"
	"github.com/stretchr/testify/assert"
)

type HelloWorldHelmChart struct {
	HelmRenderer
}

// SetDefaults implements lib.Overlay.
func (h *HelloWorldHelmChart) SetDefaults() {
	h.Name = "hello-world"
	h.Namespace = "default"
	h.ReleaseName = "hello-world"
	h.Version = "0.1.0"
	h.Repo = "https://helm.github.io/examples"
}

var _ lib.Overlay = &HelloWorldHelmChart{}

func TestHelloWorldHelmChart(t *testing.T) {
	lib.BuildOverlayWithoutOverrides[HelloWorldHelmChart]()
}

func TestReplaceValues(t *testing.T) {
	h := HelmRenderer{}

	h.OverrideValues(map[string]any{
		"foo": "bar",
		"nested": map[string]any{
			"foo": "bar",
		},
		"envs": []map[string]any{{
			"name":  "FOO",
			"value": "bar",
		}, {
			"name":  "FOO2",
			"value": "bar",
		}},
	})

	h.ReplaceValues("foo", "bar-bar")
	h.ReplaceValues("nested.foo", "bar-nested")
	h.ReplaceValues("envs.[name=FOO2].value", "bar2")
	assert.Equal(t, map[string]any{
		"foo": "bar-bar",
		"nested": map[string]any{
			"foo": "bar-nested",
		},
		"envs": []any{
			map[string]any{
				"name":  "FOO",
				"value": "bar",
			},
			map[string]any{
				"name":  "FOO2",
				"value": "bar2",
			},
		},
	}, h.values)
}
