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
	"github.com/niemal/uman"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	handleFiles   = 0
	handlePreview = 1
)

func loggingHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %q\n", r.Method, r.URL.String())
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// Middleware for requiring login on certain pages.
type loginHandler struct {
	handler http.Handler
	userMan *uman.UserManager
}

// newLoginHandler Creates a new LoginHandler, which requires logins
// from all visiting users or redirects them to /login/
func newLoginHandler(um *uman.UserManager) *loginHandler {
	return &loginHandler{
		userMan: um,
	}
}

// Simple syntax candy method to ensure that users are logged in whenever accessing
// protected Shim pages.
func (l *loginHandler) authHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := l.userMan.GetHTTPSession(w, r)

		if session.IsLogged() {
			h.ServeHTTP(w, r)
			return
		}

		redirectTarget := fmt.Sprintf("%s/login/?redirect=%s%s&warn=yes",
			shimAssets.basepath, shimAssets.basepath, url.QueryEscape(r.URL.String()))
		log.Printf("Not logged in! Redirecting to %s\n", redirectTarget)
		http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
	})
}

func (l *loginHandler) dynamicPreviewHandler(w http.ResponseWriter, r *http.Request, mode int) {
	session := l.userMan.GetHTTPSession(w, r)

	if !session.IsLogged() {
		log.Println("not logged in!")
		http.Error(w, "You must log in first!", http.StatusUnauthorized)
		return
	}

	site := findUserSite(w, r)
	if site == nil {
		http.Error(w, "We don't know what site to show you... Sorry", http.StatusInternalServerError)
		return
	}

	filename := strings.TrimLeft(r.URL.Path, "/")

	var location http.Dir
	if mode == handlePreview {
		location = http.Dir(filepath.Join(site.Location, "preview"))
	} else {
		location = http.Dir(filepath.Join(site.Location, "static", "files"))
	}

	file, err := location.Open(filename)
	if err != nil {
		if file != nil {
			file.Close()
		}
		log.Printf("Couldn't find preview at %s\n", location)
		http.Error(w, "File not found! :(", http.StatusNotFound)
		return
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		http.Error(w, "File not found", http.StatusNotFound)
	} else if info.IsDir() {
		file.Close()
		file, err = location.Open(filepath.Join(filename, "index.html"))

		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Could not find index file to serve", http.StatusNotFound)
			} else {
				http.Error(w, "Had an issue serving you the file: "+err.Error(), http.StatusInternalServerError)
			}
		}
	}

	if err != nil || file == nil {
		return
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	file.Close()
}

func panicHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from error: %v\n", r)

				tmpWrapper := new(WebWrapper)
				tmpWrapper.FailedMessage(fmt.Sprint(r))
				tmpWrapper.Base = shimAssets.baseurl
				tmpWrapper.URL = req.URL.String()

				w.WriteHeader(http.StatusInternalServerError)

				renderPage(w, "errorPage", tmpWrapper)
			}
		}()

		h.ServeHTTP(w, req)
	})
}
