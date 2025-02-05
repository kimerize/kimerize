package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestModifyAs(t *testing.T) {
	rl := BuildKustomization(kusttypes.Kustomization{
		ConfigMapGenerator: []kusttypes.ConfigMapArgs{{
			GeneratorArgs: kusttypes.GeneratorArgs{
				Name:          "foo",
				KvPairSources: kusttypes.KvPairSources{LiteralSources: []string{"foo=bar"}},
				Options:       &kusttypes.GeneratorOptions{DisableNameSuffixHash: true},
			},
		}},
	}, NoFilesystem())

	ModifyAs(rl.resources[0], func(cm *corev1.ConfigMap) {
		cm.Data["foo1"] = "bar1"
	})
	newField, err := rl.resources[0].rnode().GetFieldValue("data.foo1")
	assert.NoError(t, err)
	assert.Equal(t, "bar1", newField)
}

func TestModifyHashSuffixedResource(t *testing.T) {
	fsBuilder := func(fs filesys.FileSystem) error {
		return fs.WriteFile("pod.yaml", []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test
spec:
  containers:
    - name: test
      image: test
      envFrom:
        - configMapRef:
            name: foo
  `))
	}
	rm := BuildKustomization(kusttypes.Kustomization{
		Resources: []string{"pod.yaml"},
		ConfigMapGenerator: []kusttypes.ConfigMapArgs{{
			GeneratorArgs: kusttypes.GeneratorArgs{
				Name:          "foo",
				KvPairSources: kusttypes.KvPairSources{LiteralSources: []string{"foo=bar"}},
			},
		}},
	},
		fsBuilder,
	)

	assert := func(name string, newField string) {
		assert.Equal(t, 2, len(rm.resources))
		for _, r := range rm.resources {
			if r.rnode().GetKind() == "Pod" {
				value, err := yaml.PathGetter{
					Path: []string{"spec", "containers", "[name=test]", "envFrom", "0", "configMapRef", "name"},
				}.Filter(r.rnode())
				assert.NoError(t, err)
				assert.Equal(t, name, value.Document().Value)
			} else if r.rnode().GetKind() == "ConfigMap" {
				assert.Equal(t, name, r.rnode().GetName())
				value, _ := yaml.PathGetter{
					Path: []string{"data", "foo1"},
				}.Filter(r.rnode())
				if value == nil {
					value = yaml.NewScalarRNode("")
				}
				assert.Equal(t, newField, value.Document().Value)
			}
		}
	}

	assert("foo-798k5k7g9f", "")

	ModifyHashSuffixedResource(rm, types.NamespacedName{Name: "foo"}, func(r *corev1.ConfigMap) {
		r.Data["foo1"] = "bar1"
	})

	assert("foo-6g8k979h6t", "bar1")
}
