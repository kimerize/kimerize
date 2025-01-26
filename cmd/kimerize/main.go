package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/kimerize/kimerize/lib"
	"github.com/ztrue/tracerr"
	"golang.org/x/tools/go/packages"
)

func main() {
	target, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current working directory: %v\n", err)
	}
	// TODO: use cobra
	if len(os.Args) > 1 {
		if filepath.IsAbs(os.Args[1]) {
			target = os.Args[1]
		} else {
			target = filepath.Join(target, os.Args[1])
		}
	}
	log.Println("Processing directory:", target)

	var goDirs []string
	err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			matches, err := filepath.Glob(filepath.Join(path, "*.go"))
			if err != nil {
				return err
			}
			if len(matches) > 0 {
				goDirs = append(goDirs, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error walking directory tree: %v", err)
	}

	packages, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedModule, Dir: target}, goDirs...)
	if err != nil {
		log.Fatalf("Error loading roots: %v\n", err)
	}

	if err := processPackages(packages); err != nil {
		log.Fatalf("Error processing packages: %v\n", err)
	}
}

func processPackages(packages []*packages.Package) error {
	var errs []error

	for _, p := range packages {
		if p.Name != "main" {
			continue
		}
		log.Printf("Processing package: %s", p.PkgPath)
		if err := processPackage(p); err != nil {
			errs = append(errs, fmt.Errorf("error processing package %s: %w", p.PkgPath, err))
			continue
		}
	}
	return errors.Join(errs...)
}

func getPackageResources(pl *plugin.Plugin) (*lib.ResourceList, error) {
	symbol, err := pl.Lookup("Resources")
	if err != nil {
		return nil, err
	}

	resources, ok := symbol.(*lib.ResourceList)
	if !ok {
		return nil, fmt.Errorf("unexpected symbol signature")
	}
	return resources, nil
}

func getPackagePublisher(pl *plugin.Plugin) (lib.PackagePublisher, error) {
	symbol, err := pl.Lookup("Publisher")
	if err != nil {
		return nil, err
	}

	publisher, ok := symbol.(*lib.PackagePublisher)
	if !ok {
		return nil, fmt.Errorf("unexpected symbol signature")
	}
	return *publisher, nil
}

func printStackTrace(p *packages.Package, err tracerr.Error) {
	var frames []tracerr.Frame
	for _, f := range err.StackTrace() {
		if strings.Contains(f.Path, p.Module.Dir) {
			frames = append(frames, f)
		}
	}
	tracerr.PrintSourceColor(tracerr.CustomError(err, frames))
}

func processPackage(pkg *packages.Package) error {
	tmpDir, err := os.MkdirTemp("", "plugin-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	pluginPath := filepath.Join(tmpDir, "plugin.so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-gcflags=all=-N -l", "-o", pluginPath, pkg.Dir)
	cmd.Dir = pkg.Dir

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build error: %v\n%s", err, out)
	}

	pl, err := plugin.Open(pluginPath)
	if err != nil {
		return err
	}

	resources, err := getPackageResources(pl)
	if err != nil {
		return err
	}

	if errs := resources.Errors(); errs != nil {
		for _, e := range errs {
			printStackTrace(pkg, e)
		}
		return fmt.Errorf("error generating package")
	}

	publisher, err := getPackagePublisher(pl)
	if err != nil {
		return err
	}

	err = publisher.Publish(lib.Package{
		Resources: *resources,
		Package:   pkg,
	})
	if err != nil {
		return fmt.Errorf("failed to publish package: %w", err)
	}
	return nil
}
