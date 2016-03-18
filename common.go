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
	"regexp"
	"strings"
	"unicode"
)

// Hashmap of which users are on which sites.
var userSites map[string]string

// Find which site a user is currently viewing. If the user isn't found in
// userSites, then just return the default site.
func findUserSite(username string) *Site {
	if sitename, ok := userSites[username]; ok {
		// find site by that name then return it
		for _, s := range allSites {
			if s.ShortName() == sitename {
				return s
			}
		}
		return allSites[0]
	}

	defaultSite := allSites[0]
	userSites[username] = defaultSite.ShortName()
	return defaultSite
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

	return newSlug
}
