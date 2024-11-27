package files

import (
	"errors"
	"github.com/pufferpanel/pufferpanel/v3/utils"
	"io/fs"
	"os"
	"path/filepath"
)

type FileServer interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS

	Prefix() string

	Stat(name string) (fs.FileInfo, error)
	Mkdir(path string, mode os.FileMode) error
	MkdirAll(path string, mode os.FileMode) error
	OpenFile(path string, flags int, mode os.FileMode) (*os.File, error)
	Remove(path string) error
	Rename(source, target string) error
	RemoveAll(path string) error
	Glob(pattern string) ([]string, error)

	Close() error
}

type fileServer struct {
	dir  string
	root *os.File

	uid int
	gid int
}

func NewFileServer(prefix string, uid, gid int) (FileServer, error) {
	f := &fileServer{dir: prefix, uid: uid, gid: gid}
	var err error
	f.root, err = f.resolveRootFd()
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (sfp *fileServer) Prefix() string {
	return sfp.dir
}

func (sfp *fileServer) Open(name string) (fs.File, error) {
	return sfp.OpenFile(name, os.O_RDONLY, 0644)
}

func (sfp *fileServer) Stat(name string) (fs.FileInfo, error) {
	f, err := sfp.Open(name)
	if err != nil {
		return nil, err
	}
	defer utils.Close(f)
	return f.Stat()
}

func (sfp *fileServer) ReadDir(name string) ([]fs.DirEntry, error) {
	folder, err := sfp.OpenFile(name, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}
	defer utils.Close(folder)

	return folder.ReadDir(0)
}

func (sfp *fileServer) Glob(pattern string) ([]string, error) {
	parent := filepath.Base(pattern)
	if parent == pattern {
		parent = "."
	}

	files, err := sfp.ReadDir(parent)

	if err != nil {
		return nil, err
	}

	results := make([]string, 0)
	for _, v := range files {
		if matches, _ := filepath.Match(pattern, v.Name()); matches {
			results = append(results, v.Name())
		}
	}
	return results, nil
}

// shorten maps name, which should start with f.dir, back to the suffix after f.dir.
func (sfp *fileServer) shorten(name string) (rel string, ok bool) {
	if name == sfp.dir {
		return ".", true
	}
	if len(name) >= len(sfp.dir)+2 && name[len(sfp.dir)] == '/' && name[:len(sfp.dir)] == sfp.dir {
		return name[len(sfp.dir)+1:], true
	}
	return "", false
}

// fixErr shortens any reported names in PathErrors by stripping f.dir.
func (sfp *fileServer) fixErr(err error) error {
	var e *fs.PathError
	if errors.As(err, &e) {
		if short, ok := sfp.shorten(e.Path); ok {
			e.Path = short
		}
	}
	return err
}

func (sfp *fileServer) resolveRootFd() (*os.File, error) {
	return os.Open(sfp.dir)
}

func (sfp *fileServer) Close() error {
	return sfp.root.Close()
}
