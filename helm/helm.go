package helm

import (
	"fmt"

	"dario.cat/mergo"
	"github.com/kimerize/kimerize/lib"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/utils"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type HelmRenderer struct {
	// TODO: support embedding a helm chart
	// EmbeddedChart embed.FS
	values       map[string]any
	Name         string
	Namespace    string
	ReleaseName  string
	Version      string
	Repo         string
	ValuesMerge  string
	IncludeCRDs  bool
	SkipHooks    bool
	ApiVersions  []string
	KubeVersion  string
	NameTemplate string
	SkipTests    bool
	Debug        bool
}

// Transform implements lib.Overlay.
func (h HelmRenderer) Transform(items *lib.ResourceList) {
	items.Absorb(
		lib.BuildKustomization(
			types.Kustomization{
				HelmCharts: []types.HelmChart{{
					Name:         h.Name,
					Namespace:    h.Namespace,
					ReleaseName:  h.ReleaseName,
					Version:      h.Version,
					Repo:         h.Repo,
					ValuesMerge:  h.ValuesMerge,
					IncludeCRDs:  h.IncludeCRDs,
					SkipHooks:    h.SkipHooks,
					ApiVersions:  h.ApiVersions,
					KubeVersion:  h.KubeVersion,
					NameTemplate: h.NameTemplate,
					SkipTests:    h.SkipTests,
					Debug:        h.Debug,
					ValuesInline: h.values,
				}},
			}, lib.NoFilesystem(),
		),
	)
}

func (h *HelmRenderer) OverrideValues(override map[string]any) {
	mergo.Merge(&h.values, override)
}

// ReplaceValues replaces the values at the given path with the given value.
// Path is a string with the path to the value to replace, e.g. "spec.template.spec.containers.[name=nginx].image"
// It uses the same path format as [kustomize replacements](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/replacements/#field-path-format).
func (h *HelmRenderer) ReplaceValues(path string, v any) {
	rnode, err := yaml.FromMap(h.values)
	if err != nil {
		panic(err)
	}

	bytes, err := yaml.Marshal(v)
	if err != nil {
		lib.FailOnError(fmt.Errorf("value cannot be serialized as yaml: %w", err))
	}

	value, err := yaml.Parse(string(bytes))
	if err != nil {
		panic(err)
	}

	targetFieldList, err := rnode.Pipe(&yaml.PathMatcher{Path: utils.SmarterPathSplitter(path, "."), Create: value.YNode().Kind})
	if err != nil {
		lib.FailOnError(fmt.Errorf("failed to find finds: %w", err))
	}

	targetFields, err := targetFieldList.Elements()
	if err != nil {
		panic(err)
	}
	if len(targetFields) == 0 {
		lib.FailOnError(fmt.Errorf("failed to match any fields"))
	}

	for _, targetField := range targetFields {
		if targetField.YNode().Kind == yaml.ScalarNode {
			// For scalar, only copy the value (leave any type intact to auto-convert int->string or string->int)
			targetField.YNode().Value = value.YNode().Value
		} else {
			targetField.SetYNode(value.YNode())
		}
	}

	m, err := rnode.Map()
	if err != nil {
		panic(err)
	}
	h.values = m
}
