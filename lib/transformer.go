package lib

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"

	"github.com/GoogleContainerTools/kpt-functions-catalog/functions/go/search-replace/searchreplace"
	"github.com/go-logr/logr"
	"github.com/google/k8s-digester/pkg/resolve"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kustomize_hasher "sigs.k8s.io/kustomize/api/hasher"
	"sigs.k8s.io/kustomize/api/konfig"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/utils"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Overlay interface {
	Transform(items *ResourceList)
	SetDefaults()
}

type overlay[T any, P interface {
	*T
	Overlay
}] struct {
	config P
}

func setDefaults(o Overlay) {
	v := reflect.ValueOf(o)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("Overlay config must be a struct")
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		v := field.Addr().Interface()
		if g, ok := v.(Overlay); ok {
			setDefaults(g)
		}
	}
	o.SetDefaults()
}

func BuildOverlayWithOverrides[T any, P interface {
	*T
	Overlay
}](override func(*T)) ResourceList {
	t := *new(T)
	var p P = &t
	setDefaults(p)
	if override != nil {
		override(&t)
	}
	overlay := overlay[T, P]{
		config: p,
	}
	return overlay.Generate()
}

func BuildOverlayWithoutOverrides[T any, P interface {
	*T
	Overlay
}]() ResourceList {
	return BuildOverlayWithOverrides[T, P](nil)
}

func Aggregate(resources ...ResourceList) ResourceList {
	result := NewResourceList()
	for _, r := range resources {
		result.Absorb(r)
	}
	return *result
}

type Transformer func(*ResourceList)

func KindTransformer[T any](f func(*T)) Transformer {
	return func(rl *ResourceList) {
		FilteredTransformer(KindMatcher[T](), func(r *ResourceList) {
			r.ForEach(func(r *Resource) {
				ModifyAs(r, f)
			})
		})
	}
}

// TODO: Optionally add files
func KustomizeComponentTransformer(k kusttypes.Kustomization) Transformer {
	k.Kind = "Component"
	const idAnnotation = "internal.kimerize.io/id"
	return func(rl *ResourceList) {
		files := map[string]string{}
		componentDir := "component"
		pk := kusttypes.Kustomization{
			Components: []string{
				componentDir,
			},
		}
		counter := 0
		rl.ForEach(func(r *Resource) {
			r.SetAnnotation(idAnnotation, fmt.Sprint(counter))
			filename := fmt.Sprintf("resource-%d.yaml", counter)
			files[filename] = r.rnode().MustString()
			pk.Resources = append(pk.Resources, filename)
			counter++
		})
		newRL := BuildKustomization(pk, func(fs filesys.FileSystem) error {
			for name, content := range files {
				if err := fs.WriteFile(name, []byte(content)); err != nil {
					return err
				}
			}
			fs.MkdirAll(componentDir)
			kBytes, err := yaml.Marshal(k)
			if err != nil {
				return err
			}
			err = fs.WriteFile(filepath.Join(componentDir, konfig.RecognizedKustomizationFileNames()[0]), kBytes)
			if err != nil {
				return err
			}
			return nil
		})

		// Update resources with new resources
		rl.ForEach(func(r *Resource) {
			ra := r.Annotation(idAnnotation)
			newRL.ForEach(func(nr *Resource) {
				if nr.Annotation(idAnnotation) == ra {
					r.object = nr.object
				}
			})
		})

		// Remove resources that are not in the new resource list
		rl.RemoveAll(func(r *Resource) bool {
			ra := r.Annotation(idAnnotation)
			found := false
			newRL.ForEach(func(nr *Resource) {
				if nr.Annotation(idAnnotation) == ra {
					found = true
				}
			})
			return !found
		})

		rl.ForEach(func(r *Resource) {
			r.ClearAnnotation(idAnnotation)
		})

		// Add new resources
		newRL.ForEach(func(r *Resource) {
			if r.Annotation(idAnnotation) == "" {
				rl.Append(*r)
			}
		})
	}
}

func RegexReplaceTransformer(pattern string, replacement string) Transformer {
	return func(rl *ResourceList) {
		filter := searchreplace.SearchReplace{
			ByValueRegex: pattern,
			PutValue:     replacement,
		}

		rl.ForEach(func(r *Resource) {
			ModifyAs(r, func(rnode *yaml.RNode) {
				_, err := filter.Filter([]*yaml.RNode{rnode})
				FailOnError(err)
			})
		})
	}
}

func ValueReplaceTransformer(value string, replacement string) Transformer {
	return func(rl *ResourceList) {
		filter := searchreplace.SearchReplace{
			ByValue:  value,
			PutValue: replacement,
		}

		rl.ForEach(func(r *Resource) {
			ModifyAs(r, func(rnode *yaml.RNode) {
				_, err := filter.Filter([]*yaml.RNode{rnode})
				FailOnError(err)
			})
		})
	}
}

var hasher = &kustomize_hasher.Hasher{}

func HashSuffixedResourceTransformer(name string, t Transformer) Transformer {
	return func(rl *ResourceList) {
		nameMappings := map[k8stypes.NamespacedName]string{}
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

			nameMappings[k8stypes.NamespacedName{Name: oldName, Namespace: rnode.GetNamespace()}] = newName
		})
		for key, newName := range nameMappings {
			rl.ApplyTransformer(FilteredTransformer(
				NamespaceMatcher(key.Namespace),
				ValueReplaceTransformer(key.Name, newName),
			))
		}
	}
}

func ReplacePathsTransformer(path string, v any) Transformer {
	return func(rl *ResourceList) {
		rl.ForEach(func(r *Resource) {
			ModifyAs(r, func(rnode *yaml.RNode) {
				bytes, err := yaml.Marshal(v)
				if err != nil {
					FailOnError(fmt.Errorf("value cannot be serialized as yaml: %w", err))
				}

				value, err := yaml.Parse(string(bytes))
				if err != nil {
					panic(err)
				}

				targetFieldList, err := rnode.Pipe(&yaml.PathMatcher{Path: utils.SmarterPathSplitter(path, "."), Create: value.YNode().Kind})
				if err != nil {
					FailOnError(fmt.Errorf("failed to find finds: %w", err))
				}

				targetFields, err := targetFieldList.Elements()
				if err != nil {
					panic(err)
				}
				if len(targetFields) == 0 {
					FailOnError(fmt.Errorf("failed to match any fields"))
				}

				for _, targetField := range targetFields {
					if targetField.YNode().Kind == yaml.ScalarNode {
						// For scalar, only copy the value (leave any type intact to auto-convert int->string or string->int)
						targetField.YNode().Value = value.YNode().Value
					} else {
						targetField.SetYNode(value.YNode())
					}
				}
			})
		})
	}
}

type Matcher func(*Resource) bool

func FilteredTransformer(f Matcher, t Transformer) Transformer {
	return func(rl *ResourceList) {
		rl.ForEach(func(r *Resource) {
			if f(r) {
				r.ApplyTransformer(t)
			}
		})
	}
}

func NamespaceMatcher(ns string) Matcher {
	return func(r *Resource) bool {
		return r.Namespace() == ns
	}
}

func NameMatcher(name string) Matcher {
	return func(r *Resource) bool {
		return r.Name() == name
	}
}

func AndMatcher(matchers ...Matcher) Matcher {
	return func(r *Resource) bool {
		for _, m := range matchers {
			if !m(r) {
				return false
			}
		}
		return true
	}
}

func OrMatcher(matchers ...Matcher) Matcher {
	return func(r *Resource) bool {
		for _, m := range matchers {
			if m(r) {
				return true
			}
		}
		return false
	}
}

func NotMatcher(matcher Matcher) Matcher {
	return func(r *Resource) bool {
		return !matcher(r)
	}
}

func LabelMatcher(key, value string) Matcher {
	return func(r *Resource) bool {
		rnode := r.rnode()
		labels := rnode.GetLabels()
		lv, ok := labels[key]
		return ok && lv == value
	}
}

func KindMatcher[T any]() Matcher {
	kind := reflect.TypeOf((*T)(nil)).Elem().Name()
	return func(r *Resource) bool {
		return r.rnode().GetKind() == kind
	}
}

func Transform(resources ResourceList, transformers ...Transformer) ResourceList {
	for _, t := range transformers {
		t(&resources)
	}
	return resources
}

type buildOverlayOverrides[T any, P interface {
	*T
	Overlay
}] struct {
	override func(*T)
}

func WithOverrides[T any, P interface {
	*T
	Overlay
}](override func(*T)) buildOverlayOverrides[T, P] {
	return buildOverlayOverrides[T, P]{
		override: override,
	}
}

func WithoutOverrides[T any, P interface {
	*T
	Overlay
}]() buildOverlayOverrides[T, P] {
	return buildOverlayOverrides[T, P]{}
}

type transform struct {
	transform func(*ResourceList)
}

type Tran interface {
}

func WithTransform(f func(*ResourceList)) transform {
	return transform{
		transform: f,
	}
}

type dummyOverlayConfig struct {
	Kind string
}

func (dummyOverlayConfig) Transform(items *ResourceList) {}

func (d *dummyOverlayConfig) SetDefaults() {
	d.Kind = "dummy"
}

var _ Overlay = &dummyOverlayConfig{}

func (t overlay[T, P]) Generate() ResourceList {
	return generate(t.config)
}

func generate(o Overlay) ResourceList {
	v := reflect.ValueOf(o)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("Overlay config must be a struct")
	}

	result := NewResourceList()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		v := field.Addr().Interface()
		if g, ok := v.(Overlay); ok {
			result.Absorb(generate(g))
		}
	}
	o.Transform(result)
	return *result
}

func DigestImages(rl *ResourceList) {
	rl.ForEach(func(r *Resource) {
		ModifyAs(r, func(r *yaml.RNode) {
			resolve.ImageTags(context.TODO(), logr.Discard(), nil, r, nil)
		})
	})
}
