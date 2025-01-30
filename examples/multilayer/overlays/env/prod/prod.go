package prod

import (
	myteam "github.com/kimerize/kimerize/examples/multilayer/overlays/teams/my-team"
	. "github.com/kimerize/kimerize/lib"
)

type ProdOverlay struct {
	Prod   string
	MyTeam myteam.MyTeamOverlay
}

var _ Overlay = &ProdOverlay{}

func (config *ProdOverlay) Transform(rl *ResourceList) {
	rl.ForEach(func(r *Resource) {
		r.SetLabel("prod", config.Prod)
	})
}

func (config *ProdOverlay) SetDefaults() {
	config.Prod = "prod"
}
