package deployment

import (
	. "github.com/kimerize/kimerize/lib"
	"sigs.k8s.io/kustomize/api/resmap"
)

type TestOverlay struct {
	Foo string
}

var _ Transformer = TestOverlay{}

func (config TestOverlay) Transform(rm resmap.ResMap) {
	for _, r := range rm.Resources() {
		labels := r.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["foo"] = config.Foo
		r.SetLabels(labels)
	}
}

var Resources = Overlay[TestOverlay]{}
