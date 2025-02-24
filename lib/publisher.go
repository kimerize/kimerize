package lib

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kustomize/v5/commands/build"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// type Package struct {
// 	Resources ResourceList
// 	*packages.Package
// }

type PackagePublisher interface {
	Publish(ResourceList) error
}

const (
	dotKimerizeFile = ".kimerize"
)

type kustomizePublisher struct {
}

func KustomizePublisher() PackagePublisher {
	return kustomizePublisher{}
}

var _ PackagePublisher = kustomizePublisher{}

// Publish implements PackagePublisher.
func (k kustomizePublisher) Publish(resources ResourceList) error {
	rm := resmap.New()
	resources.ForEach(func(r *Resource) {
		rm.Append(&resource.Resource{
			RNode: NewFromResource[yaml.RNode](*r),
		})
	})
	outputDir := localPathOutput()

	// Check if directory is empty or contains .kimerize file
	entries, err := os.ReadDir(outputDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(entries) != 0 && !slices.ContainsFunc(entries, func(e os.DirEntry) bool {
		return e.Name() == dotKimerizeFile
	}) {
		return fmt.Errorf("directory %s is not empty and not a previously published dir", outputDir)
	}

	if err := os.RemoveAll(outputDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return err
	}
	// Create a .kimerize file to mark this as a generated package
	if err := os.WriteFile(filepath.Join(outputDir, dotKimerizeFile), []byte{}, 0600); err != nil {
		return err
	}
	writer := build.MakeWriter(filesys.FileSystemOrOnDisk{})
	return writer.WriteIndividualFiles(outputDir, rm)
}

type localPackagePublisher struct {
}

// Publish implements PackagePublisher.
func (l localPackagePublisher) Publish(resources ResourceList) error {
	writer := kio.LocalPackageWriter{
		PackagePath: localPathOutput(),
	}
	nodes := []*yaml.RNode{}
	resources.ForEach(func(r *Resource) {
		nodes = append(nodes, NewFromResource[*yaml.RNode](*r))
	})
	return writer.Write(nodes)
}

func LocalPackageWriter() localPackagePublisher {
	return localPackagePublisher{}
}

var _ PackagePublisher = localPackagePublisher{}

func localPathOutput() string {
	p := GetMainPkg()
	pkgRelPath, err := filepath.Rel(p.Module.Dir, p.Dir)
	if err != nil {
		panic(err)
	}
	return filepath.Join(p.Module.Dir, "zz_generated", pkgRelPath)
}
