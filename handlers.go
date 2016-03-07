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
	"github.com/niemal/uman"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

func loggingHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %q\n", r.Method, r.URL.String())
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// BUG: This is horribly broken. Fix it please.
func timeoutHandler(next http.Handler) http.Handler {
	return http.Handler(http.TimeoutHandler(next, time.Duration(15*time.Second), "Sorry, but we took too long to handle your request. Sorry?"))
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

		log.Println("Not logged in! Redirecting to login page.")
		http.Redirect(w, r, "/login/?redirect="+url.QueryEscape(r.URL.String())+"&warn=yes", http.StatusSeeOther)
	})
}

// previewStripPrefix is combined with dynamicPreviewHandler
func previewStripPrefix(h http.Handler) http.Handler {
	return http.StripPrefix("/preview/", h)
}

func (l *loginHandler) dynamicPreviewHandler(w http.ResponseWriter, r *http.Request) {
	session := l.userMan.GetHTTPSession(w, r)

	if session.IsLogged() {
		site := findUserSite(session.User)
		if site != nil {
			filename := "index.html"
			if len(r.URL.RequestURI()) > 0 {
				filename = r.URL.RequestURI()
			}

			location := http.Dir(filepath.Join(site.Location(), "preview"))
			file, err := location.Open(filename)
			if err != nil {
				// 404
				log.Printf("Couldn't find %s\n", location)
				return
			}
			defer file.Close()

			info, err := file.Stat()
			check(err)

			if info.IsDir() {
				file.Close()
				file, _ = location.Open(filepath.Join(filename, "index.html"))
				info, err = file.Stat()
				defer file.Close()
			}

			// detect if folder, then serve index.html
			log.Printf("serving from %s\n", location)
			http.ServeContent(w, r, info.Name(), info.ModTime(), file)
			return
		}
	}

	log.Println("not logged in!")

}
