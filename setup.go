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
	"github.com/spf13/viper"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
)

// Location of assets on disk for Shim
type assets struct {
	root      string
	sites     string
	templates string
	static    string
	themes    string
	url       *url.URL
}

// Copy a file at `src` to `dest`. Panic if there are any errors.
func copyFile(src, dst string) {
	in, err := os.Open(src)
	checkReason(err, "Could not open source file")
	defer in.Close()
	out, err := os.Create(dst)
	checkReason(err, "Could not create destination file")
	defer out.Close()

	_, err = io.Copy(out, in)
	checkReason(err, fmt.Sprintf("Couldn't copy file. Please delete %s and try again.", dst))
}

func setupConfig() {
	// if there is no config.toml
	if _, err := os.Stat("config.toml"); os.IsNotExist(err) {
		if _, err = os.Stat("config.toml.example"); os.IsNotExist(err) {
			// both files don't exist. Please try again
			log.Fatal("Sorry, but both config.toml and config.toml.example don't exists. Please download these files and try again.")
		}

		copyFile("config.toml.example", "config.toml")
		fmt.Println("config file copied because none already exists")
	} else {
		fmt.Println("detected existing config file")
	}
}

// findPrimarySite Finds the first site that is enabled, and returns it's name
// as a string `name`. If there are no sites available, returns an error `err`.
func findPrimarySite() (name string, err error) {
	err = nil
	sites := viper.GetStringSlice("sites.all")
	for _, name = range sites {
		enabledKey := fmt.Sprintf("sites.%s.enabled", name)
		viper.SetDefault(enabledKey, false)

		if viper.GetBool(enabledKey) {
			fmt.Printf("%s is enabled!\n", name)
			return
		}
	}

	name = ""

	if len(sites) == 0 {
		err = fmt.Errorf("No sites are available.")
		return
	}

	err = fmt.Errorf("No sites are enabled.")
	return
}

// Set up the site with the name `name` in the sites directory.
func setupSite(name string) {
	here, err := os.Getwd()
	checkReason(err, "Couldn't get current directory")

	fmt.Printf("Were are at %s\n", here)

	sitesLoc := path.Join(here, viper.GetString("sitesDir"))
	fmt.Printf("siteloc: %s\n", sitesLoc)

	// If sites folder doesn't exist, make it!
	if _, err := os.Stat(sitesLoc); os.IsNotExist(err) {
		err = os.MkdirAll(sitesLoc, 0755)
		checkReason(err, "Could not create sites directory.")
	}

	// check if site already exists
	testLoc := path.Join(here, "sites", name)
	if _, dirError := os.Stat(testLoc); !os.IsNotExist(dirError) {
		log.Println("site already exists. Let's get out")
		return
	}

	// create hugo site
	hugoPath, err := exec.LookPath("hugo")
	checkReason(err, "Couldn't find hugo. Make sure it's in your PATH")

	cmd := exec.Command(hugoPath, "new", "site", path.Join(sitesLoc, name))
	cmd.Dir = sitesLoc
	log.Printf("Creating new site in %s\n", cmd.Dir)
	err = cmd.Run()
	checkReason(err, "Error: couldn't create site "+name)

	log.Println("Done setting up site.")
}

func assignAssets() {
	shimAssets = new(assets)

	root, err := os.Getwd()
	checkReason(err, "Couldn't find current working directory.")

	viper.SetDefault("sitesDir", "sites")
	viper.SetDefault("templatesDir", "templates")
	viper.SetDefault("staticDir", "static")
	viper.SetDefault("themeDir", "themes")
	viper.SetDefault("baseurl", "http://127.0.0.1:8080/")

	shimAssets.root = root
	shimAssets.sites = viper.GetString("sitesDir")
	shimAssets.templates = viper.GetString("templatesDir")
	shimAssets.static = viper.GetString("staticDir")
	shimAssets.themes = viper.GetString("themeDir")

	baseurl := viper.GetString("baseurl")
	shimAssets.url, err = url.Parse(baseurl)
	if err != nil {
		log.Fatal("Invalid URL for \"baseurl\"!")
	}
}
