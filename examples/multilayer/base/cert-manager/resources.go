package certmanager

import (
	"fmt"

	. "github.com/kimerize/kimerize/lib"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type CertManager struct {
	Version string
}

var _ Overlay = &CertManager{}

// SetDefaults implements lib.OverlayConfig.
func (c *CertManager) SetDefaults() {
	c.Version = "1.6.2"
}

// Transform implements lib.OverlayConfig.
func (c CertManager) Transform(items *ResourceList) {
	items.Absorb(KustomizeBuild(".", func(fs filesys.FileSystem) error {
		return WriteKustomization(fs, types.Kustomization{
			Resources: []string{
				fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/v%s/cert-manager.yaml", c.Version),
			},
		})
	}))
}
