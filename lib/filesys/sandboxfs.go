package filesys

import (
	"errors"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type SandboxFS struct {
	sandboxSrc  string
	sandboxRoot string
	sandboxDst  string
	fs          filesys.FileSystem
}

func NewSandboxFS(fs filesys.FileSystem, path string) (*SandboxFS, error) {
	sandboxRoot, err := filesys.NewTmpConfirmedDir()
	if err != nil {
		return nil, err
	}
	return &SandboxFS{
		sandboxRoot: string(sandboxRoot),
		sandboxDst:  filepath.Join(string(sandboxRoot), path),
		sandboxSrc:  path,
		fs:          fs,
	}, nil
}

// CleanedAbs implements filesys.FileSystem.
func (s *SandboxFS) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	if !filepath.IsAbs(path) {
		return s.CleanedAbs(filepath.Join(s.sandboxDst, path))
	}

	if d, n, err := filesys.MakeFsOnDisk().CleanedAbs(path); err == nil {
		return d, n, nil
	}

	d, n, err := s.fs.CleanedAbs(strings.TrimPrefix(path, s.sandboxRoot))
	d = filesys.ConfirmedDir(filepath.Join(s.sandboxRoot, string(d)))
	return d, n, err
}

// Create implements filesys.FileSystem.
func (s *SandboxFS) Create(path string) (filesys.File, error) {
	return filesys.MakeFsOnDisk().Create(path)
}

// Exists implements filesys.FileSystem.
func (s *SandboxFS) Exists(path string) bool {
	return filesys.MakeFsOnDisk().Exists(path) || s.fs.Exists(strings.TrimPrefix(path, s.sandboxRoot))
}

// Glob implements filesys.FileSystem.
func (s *SandboxFS) Glob(pattern string) ([]string, error) {
	rs, errs := filesys.MakeFsOnDisk().Glob(pattern)
	rf, errf := s.fs.Glob(strings.TrimPrefix(pattern, s.sandboxRoot))
	for i := range rf {
		rf[i] = filepath.Join(s.sandboxRoot, rf[i])
	}
	return uniqueStrings(append(rs, rf...)), errors.Join(errs, errf)
}

// IsDir implements filesys.FileSystem.
func (s *SandboxFS) IsDir(path string) bool {
	return filesys.MakeFsOnDisk().IsDir(path) || s.fs.IsDir(strings.TrimPrefix(path, s.sandboxRoot))
}

// Mkdir implements filesys.FileSystem.
func (s *SandboxFS) Mkdir(path string) error {
	return filesys.MakeFsOnDisk().Mkdir(path)
}

// MkdirAll implements filesys.FileSystem.
func (s *SandboxFS) MkdirAll(path string) error {
	return filesys.MakeFsOnDisk().MkdirAll(path)
}

// Open implements filesys.FileSystem.
func (s *SandboxFS) Open(path string) (filesys.File, error) {
	if filesys.MakeFsOnDisk().Exists(path) {
		return filesys.MakeFsOnDisk().Open(path)
	}
	return s.fs.Open(strings.TrimPrefix(path, s.sandboxRoot))
}

// ReadDir implements filesys.FileSystem.
func (s *SandboxFS) ReadDir(path string) ([]string, error) {
	if filesys.MakeFsOnDisk().Exists(path) {
		return filesys.MakeFsOnDisk().ReadDir(path)
	}
	r, err := s.fs.ReadDir(strings.TrimPrefix(path, s.sandboxRoot))
	for i := range r {
		r[i] = filepath.Join(s.sandboxRoot, r[i])
	}
	return r, err
}

// ReadFile implements filesys.FileSystem.
func (s *SandboxFS) ReadFile(path string) ([]byte, error) {
	if filesys.MakeFsOnDisk().Exists(path) {
		return filesys.MakeFsOnDisk().ReadFile(path)
	}
	return s.fs.ReadFile(strings.TrimPrefix(path, s.sandboxRoot))
}

// RemoveAll implements filesys.FileSystem.
func (s *SandboxFS) RemoveAll(path string) error {
	return filesys.MakeFsOnDisk().RemoveAll(path)
}

// Walk implements filesys.FileSystem.
func (s *SandboxFS) Walk(path string, walkFn filepath.WalkFunc) error {
	panic("not implemented")
}

// WriteFile implements filesys.FileSystem.
func (s *SandboxFS) WriteFile(path string, data []byte) error {
	return filesys.MakeFsOnDisk().WriteFile(path, data)
}

var _ filesys.FileSystem = &SandboxFS{}
