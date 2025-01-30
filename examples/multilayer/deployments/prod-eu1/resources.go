//go:generate go run ./
package main

import (
	"github.com/kimerize/kimerize/examples/multilayer/overlays/env/prod"
	"github.com/kimerize/kimerize/examples/multilayer/utils"
	. "github.com/kimerize/kimerize/lib"
)

func main() {
	FailOnError(
		KustomizePublisher().Publish(Transform(
			Aggregate(
				BuildOverlayWithOverrides(func(t *prod.ProdOverlay) {
					t.MyTeam.CertManager.Version = "1.5.0"
				}),
			),
			utils.CompanyTransforms()...,
		)),
	)
}
