package lib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

func TestKustomizeComponentTransformer(t *testing.T) {
	rl := NewResourceList()
	rl.Append(ResourceFrom(map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name": "nginx",
		},
		"spec": map[string]any{
			"replicas": 3,
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []map[string]any{{
						"name":  "nginx",
						"image": "nginx:1.7.9",
					}},
				},
			},
		},
	}))
	rl.Append(ResourceFrom(map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name": "nginx",
		},
		"data": map[string]any{
			"foo": "bar",
		},
	}))
	rl.ApplyTransformer(KustomizeComponentTransformer(types.Kustomization{
		Labels: []types.Label{{
			Pairs: map[string]string{
				"app": "nginx",
			},
			IncludeSelectors: true,
			IncludeTemplates: true,
		}},
		ConfigMapGenerator: []types.ConfigMapArgs{{
			GeneratorArgs: types.GeneratorArgs{
				Name:          "foo",
				KvPairSources: types.KvPairSources{LiteralSources: []string{"foo=bar"}},
				Options:       &types.GeneratorOptions{DisableNameSuffixHash: true},
			},
		}},
	}))
	assert.Equal(t, 3, len(rl.resources))
	assert.Equal(t, strings.TrimSpace(`
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
`), strings.TrimSpace(rl.resources[0].MustString()))
	assert.Equal(t, strings.TrimSpace(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  labels:
    app: nginx
  name: nginx
`), strings.TrimSpace(rl.resources[1].MustString()))
	assert.Equal(t, strings.TrimSpace(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  labels:
    app: nginx
  name: foo
`), strings.TrimSpace(rl.resources[2].MustString()))
}

func TestSearchReplaceTransformer(t *testing.T) {
	ingress := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp
  namespace: myapp
spec:
  rules:
  - host: myapp.__MY_DOMAIN__
    http:
      paths:
      - backend:
          service:
            name: myapp
            port:
              name: http
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - myapp.__MY_DOMAIN__
    secretName: myapp-tls
`
	ingressMap := make(map[string]any)
	err := yaml.Unmarshal([]byte(ingress), &ingressMap)
	assert.NoError(t, err)
	rl := NewResourceList()
	rl.Append(ResourceFrom(ingressMap))

	rl.ApplyTransformer(RegexReplaceTransformer("(.+)\\.__MY_DOMAIN__$", "${1}.example.com"))
	assert.Equal(t, strings.TrimSpace(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp
  namespace: myapp
spec:
  rules:
  - host: myapp.example.com
    http:
      paths:
      - backend:
          service:
            name: myapp
            port:
              name: http
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - myapp.example.com
    secretName: myapp-tls
`), strings.TrimSpace(rl.resources[0].MustString()))
}

func TestReplacePathsTransformer(t *testing.T) {
	ingress := `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: nginx
  - name: sidecar
    image: sidecar
`
	ingressMap := make(map[string]any)
	err := yaml.Unmarshal([]byte(ingress), &ingressMap)
	assert.NoError(t, err)
	rl := NewResourceList()
	rl.Append(ResourceFrom(ingressMap))

	rl.ApplyTransformer(ReplacePathsTransformer("spec.containers.[name=nginx].env", []corev1.EnvVar{{
		Name:  "FOO",
		Value: "bar",
	}}))
	assert.Equal(t, strings.TrimSpace(`
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - env:
    - name: FOO
      value: bar
    image: nginx
    name: nginx
  - image: sidecar
    name: sidecar
`), strings.TrimSpace(rl.resources[0].MustString()))
}
