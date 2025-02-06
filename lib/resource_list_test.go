package lib

import (
	"cmp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func assertPath(t *testing.T, path []string, rnode *yaml.RNode, want any) {
	v, err := yaml.PathGetter{
		Path: path,
	}.Filter(rnode)
	assert.NoError(t, err)
	assert.Equal(t, want, v.Document().Value)
}

func TestResourceListApplyFiltersSuffixedResource(t *testing.T) {
	rl := BuildKustomization(kusttypes.Kustomization{
		Resources: []string{"pods.yaml"},
		ConfigMapGenerator: []kusttypes.ConfigMapArgs{{
			GeneratorArgs: kusttypes.GeneratorArgs{
				Name:          "cm1",
				Namespace:     "ns1",
				KvPairSources: kusttypes.KvPairSources{LiteralSources: []string{"foo=bar"}},
			},
		}, {
			GeneratorArgs: kusttypes.GeneratorArgs{
				Name:          "cm1",
				Namespace:     "ns2",
				KvPairSources: kusttypes.KvPairSources{LiteralSources: []string{"foo=bar"}},
			},
		}},
	}, func(fs filesys.FileSystem) error {
		return fs.WriteFile("pods.yaml", []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: pod1
  namespace: ns1
spec:
  containers:
    - name: test
      image: test
      envFrom:
        - configMapRef:
            name: cm1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
  namespace: ns2
spec:
  containers:
    - name: test
      image: test
      envFrom:
        - configMapRef:
            name: cm1
        `))
	})

	rl.ApplyTransformer(
		HashSuffixedResourceTransformer("cm1",
			FilteredTransformer(
				NamespaceMatcher("ns1"),
				KindTransformer(func(cm *corev1.ConfigMap) {
					cm.Data["foo"] = "baz"
				}),
			),
		),
	)

	slices.SortFunc(rl.resources, func(a, b *Resource) int {
		return cmp.Compare(a.String(), b.String())
	})

	assert.Equal(t, 4, len(rl.resources))
	assert.Equal(t, "cm1-9922d695f7", rl.resources[0].rnode().GetName())
	assert.Equal(t, "cm1-798k5k7g9f", rl.resources[1].rnode().GetName())

	path := []string{"spec", "containers", "[name=test]", "envFrom", "0", "configMapRef", "name"}
	assertPath(t, path, rl.resources[2].rnode(), "cm1-9922d695f7")
	assertPath(t, path, rl.resources[3].rnode(), "cm1-798k5k7g9f")
}
