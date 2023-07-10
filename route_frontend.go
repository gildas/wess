package wess

import (
	"io/fs"
	"net/http"
	"path"
)

// protectedFileSystem is a wrapper for http.FileServer that does not allow directory listing
type protectedFileSystem struct {
	fs http.FileSystem
}

// Open opens the file
func (pfs protectedFileSystem) Open(filepath string) (http.File, error) {
	file, err := pfs.fs.Open(filepath)
	if err != nil {
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		index := path.Join(filepath, "index.html")
		if _, err := pfs.fs.Open(index); err != nil {
			if cerr := file.Close(); cerr != nil {
				return nil, cerr
			}
			return nil, err
		}
	}
	return file, nil
}

// AddFrontend adds a frontend to the server
//
// The frontend is a static website that will be served by the server.
func (server Server) AddFrontend(path string, rootFS fs.FS, rootPath string) error {
	websiteFS, err := fs.Sub(rootFS, rootPath)
	if err != nil {
		return err
	}
	server.webrouter.PathPrefix(path).Handler(http.StripPrefix(path, http.FileServer(protectedFileSystem{http.FS(websiteFS)})))
	return nil
}
