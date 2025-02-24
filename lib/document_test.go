package lib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/yaml"
)

func TestReplacePaths(t *testing.T) {
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
	doc := NewDocumentFrom(ingressMap)

	doc.ReplacePaths("spec.containers.[name=nginx].env", []corev1.EnvVar{{
		Name:  "FOO",
		Value: "bar",
	}})
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
`), strings.TrimSpace(NewFromDocument[*kyaml.RNode](doc).MustString()))
}

func TestRegexReplaceValues(t *testing.T) {
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
	doc := NewDocumentFrom(ingressMap)

	doc.RegexReplaceValues("(.+)\\.__MY_DOMAIN__$", "${1}.example.com")
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
`), strings.TrimSpace(NewFromDocument[*kyaml.RNode](doc).MustString()))
}
