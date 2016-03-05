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
	"strings"
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
