package lib

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ztrue/tracerr"
	"golang.org/x/tools/go/packages"
)

// TODO: Implement caching
func GetMainPkg() packages.Package {
	for _, f := range tracerr.Wrap(fmt.Errorf("")).StackTrace() {
		dir := filepath.Dir(f.Path)
		p, err := packages.Load(
			&packages.Config{
				Mode: packages.NeedName | packages.NeedFiles | packages.NeedModule,
				Dir:  dir,
			},
			dir,
		)
		for _, p := range p {
			if p.Name == "main" {
				return *p
			}
		}
		if err != nil {
			log.Fatalf("Error loading roots: %v\n", err)
		}
	}
	panic("main package not found")
}

func FailOnError(err error) {
	defer func() {
		// Getting main package might fail
		if r := recover(); r != nil {
			tracerr.Print(tracerr.Wrap(err))
			os.Exit(1)
		}
	}()
	if err != nil {
		printStackTrace(GetMainPkg(), tracerr.Wrap(err))
		os.Exit(1)
	}
}

func printStackTrace(p packages.Package, err tracerr.Error) {
	var frames []tracerr.Frame
	for _, f := range err.StackTrace() {
		if strings.Contains(f.Path, p.Module.Dir) {
			frames = append(frames, f)
		}
	}
	tracerr.PrintSourceColor(tracerr.CustomError(err, frames))
}

func localPathOutput() string {
	p := GetMainPkg()
	pkgRelPath, err := filepath.Rel(p.Module.Dir, p.Dir)
	if err != nil {
		panic(err)
	}
	return filepath.Join(p.Module.Dir, "zz_generated", pkgRelPath)
}
