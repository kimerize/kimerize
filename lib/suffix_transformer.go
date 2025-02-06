package lib

import (
	"regexp"

	"k8s.io/apimachinery/pkg/types"
	kustomize_hasher "sigs.k8s.io/kustomize/api/hasher"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/walk"
)

var hasher = &kustomize_hasher.Hasher{}

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

func HashSuffixedResourceTransformer(name string, t Transformer) Transformer {
	return func(rl *ResourceList) {
		nameMappings := map[types.NamespacedName]string{}
		re := regexp.MustCompile(`(^.+)-([a-z0-9]{10})$`)
		rl.ForEach(func(r *Resource) {
			rnode := r.rnode()
			if match := re.FindStringSubmatch(rnode.GetName()); match != nil {
				if name != match[1] {
					return
				}
				rnode.SetName(name)
				wantedHash := match[2]
				gotHash, _ := hasher.Hash(rnode)
				if wantedHash != gotHash {
					return
				}
			} else {
				return
			}

			r.ApplyTransformer(t)

			rnode = r.rnode()
			oldName := rnode.GetName()
			var newName string

			// Modify hash suffix
			ModifyAs(r, func(r *yaml.RNode) {
				newHash, err := hasher.Hash(r)
				if err != nil {
					panic(err)
				}
				r.SetName(name + "-" + newHash)
				newName = r.GetName()
			})

			nameMappings[types.NamespacedName{Name: oldName, Namespace: rnode.GetNamespace()}] = newName
		})
		for key, newName := range nameMappings {
			rl.ForEach(func(r *Resource) {
				if key.Namespace != r.rnode().GetNamespace() {
					return
				}
				ModifyAs(r, func(r *yaml.RNode) {
					_, err := walk.Walker{
						Visitor: &valueSetterWalker{value: key.Name, newValue: newName},
						Sources: walk.Sources{r},
					}.Walk()
					if err != nil {
						// TODO: handle error
					}
				})
			})
		}
	}
}
