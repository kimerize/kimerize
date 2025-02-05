package lib

import (
	"reflect"
	"regexp"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/api/hasher"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/walk"
)

var h = &hasher.Hasher{}

type valueSetterWalker struct {
	value    string
	newValue string
}

func (n *valueSetterWalker) visitCollection(elements []*yaml.RNode) error {
	for _, element := range elements {
		_, err := walk.Walker{
			Visitor: n,
			Sources: walk.Sources{element},
		}.Walk()
		if err != nil {
			return err
		}
	}
	return nil
}

// VisitList implements walk.Visitor.
func (n *valueSetterWalker) VisitList(sources walk.Sources, _ *openapi.ResourceSchema, _ walk.ListKind) (*yaml.RNode, error) {
	elements, error := sources.Dest().Elements()
	if error != nil {
		return nil, error
	}
	if err := n.visitCollection(elements); err != nil {
		return nil, err
	}
	return sources.Dest(), nil
}

// VisitMap implements walk.Visitor.
func (n *valueSetterWalker) VisitMap(sources walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	fields, error := sources.Dest().FieldRNodes()
	if error != nil {
		return nil, error
	}
	if err := n.visitCollection(fields); err != nil {
		return nil, err
	}
	return sources.Dest(), nil
}

// VisitScalar implements walk.Visitor.
func (n *valueSetterWalker) VisitScalar(sources walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	return yaml.ValueReplacer{StringMatch: n.value, Replace: n.newValue}.Filter(sources.Dest())
}

var _ walk.Visitor = &valueSetterWalker{}

func ModifyHashSuffixedResource[T any](rl ResourceList, name types.NamespacedName, fn func(*T)) {
	kind := reflect.TypeOf((*T)(nil)).Elem().Name()
	var foundName string
	var newName string
	for _, r := range rl.resources {
		rnode := r.rnode()
		if rnode.GetKind() != kind {
			continue
		}
		copy := rnode.Copy()
		re := regexp.MustCompile(`(^.+)-([a-z0-9]{10})$`)
		if match := re.FindStringSubmatch(copy.GetName()); match != nil {
			if name.Name != match[1] || rnode.GetNamespace() != name.Namespace {
				continue
			}
			copy.SetName(name.Name)
			wantedHash := match[2]
			if gotHash, _ := h.Hash(copy); wantedHash == gotHash {
				foundName = rnode.GetName()
				ModifyAs(r, fn)
				newHash, _ := h.Hash(r.rnode())
				ModifyAs(r, func(r *yaml.RNode) {
					r.SetName(name.Name + "-" + newHash)
					newName = r.GetName()
				})
			}
			break
		}
	}

	for _, r := range rl.resources {
		rnode := r.rnode()
		if rnode.GetKind() == kind && rnode.GetName() == newName {
			continue
		}
		ModifyAs(r, func(r *yaml.RNode) {
			_, err := walk.Walker{
				Visitor: &valueSetterWalker{value: foundName, newValue: newName},
				Sources: walk.Sources{r},
			}.Walk()
			if err != nil {
				// TODO: handle error
			}
		})

	}
}
