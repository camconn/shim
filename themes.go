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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// GetThemes - Find a list of all themes available in the themesDir.
func GetThemes(themesDir string) ([]string, error) {
	themes := []string{}
	files, err := ioutil.ReadDir(themesDir)
	if err != nil {
		return themes, fmt.Errorf("Could not load theme directory: %s", err.Error())
	}

	for _, f := range files {
		if f.IsDir() {
			themes = append(themes, f.Name())
		}
	}

	if _, err := os.Stat(fmt.Sprintf("%s/slim", themesDir)); len(themes) == 0 || os.IsNotExist(err) {
		// It's okay to use log.Fatal here:
		log.Fatal("Could not find the default theme!\n" +
			"Please run 'git submodule --init', then try again.")
	}

	return themes, nil
}

// DownloadTheme - Download a theme from a given git url with a custom name to
// themeFolder from themeDownloadLoc. The theme's folder will be either called
// customName, or it's default (if len(customName) == 0).
func DownloadTheme(themeFolder, themeDownloadLoc, customName string) error {
	// TODO: Implement this
	return nil
}

// ChangeTheme - Change the theme for a site siteName to themeName
func ChangeTheme(site *Site, themeName string) error {
	newThemeSrc := filepath.Join(shimAssets.root, shimAssets.themes, themeName)

	if _, err := os.Stat(newThemeSrc); os.IsNotExist(err) {
		fmt.Printf("Could not find theme folder. %s does not exist!\n", newThemeSrc)
		return err
	}

	siteThemePath := filepath.Join(site.Location, "themes")
	if _, err := os.Stat(siteThemePath); os.IsNotExist(err) {
		fmt.Printf("Could not find old theme folder at %s. It does not exist!\n", siteThemePath)
		return err
	}
	// remove all symlinks in that folder
	globString := filepath.Join(siteThemePath, "*")
	files, err := filepath.Glob(globString)
	if err != nil {
		return err
	}

	for _, name := range files {
		err = os.RemoveAll(name)
		if err != nil {
			return err
		}
	}

	// create symlink
	target := filepath.Join(siteThemePath, themeName)
	err = os.Symlink(newThemeSrc, target)
	return err
}
