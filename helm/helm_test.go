package helm

import (
	"testing"

	"github.com/kimerize/kimerize/lib"
	"sigs.k8s.io/kustomize/api/types"
)

type HelloWorldHelmChart struct {
	HelmRenderer
}

// SetDefaults implements lib.Overlay.
func (h *HelloWorldHelmChart) SetDefaults() {
	h.HelmChart = types.HelmChart{
		Name:        "hello-world",
		Namespace:   "default",
		ReleaseName: "hello-world",
		Version:     "0.1.0",
		Repo:        "https://helm.github.io/examples",
	}
}

var _ lib.Overlay = &HelloWorldHelmChart{}

func TestHelloWorldHelmChart(t *testing.T) {
	lib.BuildOverlayWithoutOverrides[HelloWorldHelmChart]()
}
