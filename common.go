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
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	siteSelectionLifespan = 3600 * 24 * 7 * 365 * 5
)

// Find which site a user is currently viewing.
func findUserSite(rw http.ResponseWriter, req *http.Request) *Site {
	var siteName string

	siteCookie, err := req.Cookie("currentSite")
	validSite := false

	userSite := allSites[0] // Default to first site

	if err == nil {
		siteName = siteCookie.Value
		for _, s := range allSites {
			if len(siteName) == len(s.ShortName()) && siteName == s.ShortName() {
				validSite = true
				userSite = s
				break
			}
		}
	}

	if !validSite {
		setUserSite(rw, req, userSite.ShortName())
	}

	return userSite
}

func setUserSite(rw http.ResponseWriter, req *http.Request, siteName string) {
	newSiteCookie := http.Cookie{
		Name:     "currentSite",
		Value:    siteName,
		Expires:  time.Unix(time.Now().Unix()+siteSelectionLifespan, 0),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(siteSelectionLifespan),
	}

	http.SetCookie(rw, &newSiteCookie)

}

// stripChars Trims the characters in `chars` from each element in a slice.
func stripChars(slice *[]string, chars string) {
	for i, v := range *slice {
		(*slice)[i] = strings.Trim(v, chars)
	}
}

// Algorithm for this function found from the following mailing list:
// https://groups.google.com/forum/#!topic/golang-nuts/-pqkICuokio
// Thanks to Paul Hankin!
func removeDuplicates(slice *[]string) {
	found := make(map[string]bool)
	j := 0
	for i, k := range *slice {
		if !found[k] {
			found[k] = true
			(*slice)[j] = (*slice)[i]
			j++
		}
	}
	*slice = (*slice)[:j]
}

var regexWhitespace = regexp.MustCompile(`\s+`)
var regexURLSafe = regexp.MustCompile(`^[a-z0-9\._\|\-\/]+$`)
var regexSlashSymbols = regexp.MustCompile(`[~%\^\#!\?\(\)&\*]+`)

// NormalizeSlug creates a page slug from a post title which can be used safely
// in both URLs as well as filenames.
func NormalizeSlug(title, archetype string) string {
	// Normalization for slugs
	newSlug := strings.TrimSpace(title)
	newSlug = strings.ToLower(newSlug)
	// compress all repeating whitespace into single space
	newSlug = regexWhitespace.ReplaceAllString(newSlug, "-")
	newSlug = regexSlashSymbols.ReplaceAllString(newSlug, "-")

	// If we aren't doing an absolute path, then remove slashes
	newSlug = strings.Replace(newSlug, "..", "", -1) // prevent sketchy stuff

	removeNonSpacing := transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r)
	})
	removeNonSafe := transform.RemoveFunc(func(r rune) bool {
		return !regexURLSafe.MatchString(string(r))
	})

	chain := transform.Chain(norm.NFD, removeNonSpacing, removeNonSafe, norm.NFC)
	newSlug, _, err := transform.String(chain, newSlug)
	if err != nil {
		log.Println("Error normalizing to slug: " + err.Error())
	}

	// Make sure to remove preceding or trailing -dashes-
	return strings.Trim(newSlug, "-")
}
