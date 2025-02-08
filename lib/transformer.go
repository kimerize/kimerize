package lib

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/google/k8s-digester/pkg/resolve"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
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
		kind := reflect.TypeOf((*T)(nil)).Elem().Name()
		rl.ForEach(func(r *Resource) {
			if r.rnode().GetKind() == kind {
				ModifyAs(r, f)
			}
		})
	}
}

// TODO: Optionally add files
func KustomizeComponentTransformer(k types.Kustomization) Transformer {
	k.Kind = "Component"
	const idAnnotation = "internal.kimerize.io/id"
	return func(rl *ResourceList) {
		files := map[string]string{}
		componentDir := "component"
		pk := types.Kustomization{
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
