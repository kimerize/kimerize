package helm

import (
	"embed"

	"github.com/kimerize/kimerize/lib"
	"sigs.k8s.io/kustomize/api/types"
)

type HelmRenderer struct {
	types.HelmChart
	FS embed.FS
}

// Transform implements lib.Overlay.
func (h HelmRenderer) Transform(items *lib.ResourceList) {
	if h.HelmChart.Repo == "" {

	}
	items.Absorb(
		lib.BuildKustomization(
			types.Kustomization{
				HelmCharts: []types.HelmChart{h.HelmChart},
			},
			lib.EmbedFilesysBuilder(h.FS),
		),
	)
}
