// SHIM - A web front end for the Hugo site generator
// Copyright (C) 2016        Cameron Conn

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const (
	embedFmt = "{{< figure src=\"files/%s\" title=\"Put Figure Name Here\" >}}"
)

// StaticFiles retrieves and returns a list of all of the statically-uploaded
// user files for this site.
func (s *Site) StaticFiles() []string {
	staticPath := filepath.Join(s.Location, "static", "files")

	if _, err := os.Stat(staticPath); os.IsNotExist(err) {
		os.Mkdir(staticPath, 0755)
	}

	staticFiles := []string{}
	staticLoc := filepath.Join(s.Location, "static", "files")

	scanFunc := func(path string, fileInfo os.FileInfo, _ error) error {
		if !fileInfo.IsDir() {
			path, err := filepath.Rel(staticLoc, path)
			if err != nil {
				return err
			}
			staticFiles = append(staticFiles, path)
		}
		return nil
	}

	err := filepath.Walk(staticPath, scanFunc)
	if err != nil {
		log.Printf("Could not load static files: %s\n", err.Error())
	}

	return staticFiles
}

// AddStaticFile saves a `file` to the `path` in this site's static directory.
// If the file already exists, then return an error.
func (s *Site) AddStaticFile(path string, uploadFile io.Reader) error {
	if _, err := s.GetStaticFile(path); err == nil {
		return fmt.Errorf("Sorry, but that file already exists.")
	}

	// TODO: This is dangerous. Maybe make sure that path is a descendant of
	// the static file path?
	newFilePath := filepath.Join(s.Location, "static", "files", path)
	fmt.Printf("New file path: %s\n", newFilePath)
	newFile, err := os.Create(newFilePath)
	defer newFile.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(newFile, uploadFile)
	return err
}

// GetStaticFile A safe method for getting a static file according to its
// relative path to the site's static file directory.
func (s *Site) GetStaticFile(path string) (http.File, error) {
	filesRoot := filepath.Join(s.Location, "static", "files")

	staticFSRoot := http.Dir(filesRoot)
	return staticFSRoot.Open(path)
}

// RemoveStaticFile A safe method for removing static files from a site's
// static file directory using the relative path
func (s *Site) RemoveStaticFile(path string) error {
	_, err := s.GetStaticFile(path)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(filepath.Join(s.Location, "static", "files", path))
	if err == nil {
		err = os.Remove(absPath)
	}

	return err
}

// GetEmbedCode Generate the embedding code for a static file used in this site
// with Hugo.
func (s *Site) GetEmbedCode(path string) string {
	embedCode := fmt.Sprintf(embedFmt, path)
	return embedCode
}
