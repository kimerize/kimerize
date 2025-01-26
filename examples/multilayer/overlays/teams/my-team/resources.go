package myteam

import (
	certmanager "github.com/kimerize/kimerize/examples/multilayer/base/cert-manager"
	. "github.com/kimerize/kimerize/lib"
)

type MyTeamOverlay struct {
	Team        string
	CertManager certmanager.CertManager
}

// SetDefaults implements lib.Transformer.
func (config *MyTeamOverlay) SetDefaults() {
	config.Team = "my-team"
}

func (config *MyTeamOverlay) Transform(resources *ResourceList) {
	resources.ForEach(func(r *Resource) {
		r.SetLabel("team", config.Team)
	})
}
