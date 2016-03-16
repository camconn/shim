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

// Simple syntax candy method to ensure that users are logged in whenever accessing
// protected Shim pages.
func authHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := um.GetHTTPSession(w, r)

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

func dynamicPreviewHandler(w http.ResponseWriter, r *http.Request) {
	session := um.GetHTTPSession(w, r)

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
				http.Error(w, "File not found! :(", http.StatusNotFound)
				return
			}
			defer file.Close()

			info, err := file.Stat()
			check(err)

			if info.IsDir() {
				file.Close()
				file, _ = location.Open(filepath.Join(filename, "index.html"))
				info, err = file.Stat()
				if err != nil {
					log.Printf("no index file!")
					http.Error(w, "Could not find index file to serve", http.StatusNotFound)
					return
				}
				defer file.Close()
			}

			// detect if folder, then serve index.html
			http.ServeContent(w, r, info.Name(), info.ModTime(), file)
		}
	} else {
		log.Println("not logged in!")
		http.Error(w, "You must log in first!", http.StatusUnauthorized)
	}

}
