package lib

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	kimerize_filesys "github.com/kimerize/kimerize/lib/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func WriteKustomization(fs filesys.FileSystem, kustomization types.Kustomization) error {
	kBytes, err := yaml.Marshal(kustomization)
	if err != nil {
		return err
	}
	err = fs.WriteFile(konfig.RecognizedKustomizationFileNames()[0], kBytes)
	if err != nil {
		return err
	}
	return nil
}

func BuildKustomization(k types.Kustomization, builder func(fs filesys.FileSystem) error) ResourceList {
	return BuildKustomizeLayer(".", func(fs filesys.FileSystem) error {
		if err := builder(fs); err != nil {
			return err
		}
		return WriteKustomization(fs, k)
	})
}

func NoFilesystem() func(filesys.FileSystem) error {
	return func(fs filesys.FileSystem) error {
		return nil
	}
}

func BuildKustomizeLayer(path string, buildFS func(fs filesys.FileSystem) error) (result ResourceList) {
	if !filepath.IsLocal(path) {
		FailOnError(fmt.Errorf("path must be local"))
	}

	inMemoryFileSystem := kimerize_filesys.MakeFsInMemory()
	if err := buildFS(inMemoryFileSystem); err != nil {
		FailOnError(err)
	}

	fs, err := kimerize_filesys.NewSandboxFS(inMemoryFileSystem, "")
	defer fs.Delete()
	FailOnError(err)

	return buildKustomizeDir(fs, path)
}

func buildKustomizeDir(fs filesys.FileSystem, path string) (result ResourceList) {
	options := krusty.MakeDefaultOptions()
	options.PluginConfig = types.EnabledPluginConfig(types.BploUseStaticallyLinked)

	tempDir, err := os.MkdirTemp("", "helm-bin-")
	FailOnError(err)
	defer os.RemoveAll(tempDir)

	helmBin := filepath.Join(tempDir, "helm")
	cwd, err := os.Getwd()
	FailOnError(err)
	helmBinScript := fmt.Sprintf(strings.TrimSpace(`
#!/bin/bash

cd %s
go tool helm "$@"
`), cwd)
	os.WriteFile(helmBin, []byte(helmBinScript), 0755)

	options.PluginConfig.HelmConfig.Enabled = true
	options.PluginConfig.HelmConfig.Command = helmBin

	options.PluginConfig.FnpLoadingOptions.EnableExec = true
	k := krusty.MakeKustomizer(options)
	rm, err := k.Run(fs, filepath.Join(path, "."))
	if err != nil {
		FailOnError(err)
	}
	for _, r := range rm.Resources() {
		result.Append(NewResource(r.RNode))
	}
	return
}

func BuildKustomizeDir(path string) ResourceList {
	return buildKustomizeDir(filesys.MakeFsOnDisk(), path)
}

func EmbedFilesysBuilder(embedFS embed.FS) func(filesys.FileSystem) error {
	return func(targetFS filesys.FileSystem) error {
		return fs.WalkDir(embedFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return targetFS.MkdirAll(path)
			} else {
				content, err := embedFS.ReadFile(path)
				if err != nil {
					return err
				}
				return targetFS.WriteFile(path, content)
			}
		})
	}
}
