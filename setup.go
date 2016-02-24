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
	"os"
	"os/exec"
	"path"
)

// Simple utility to copy files
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

func setupTestSite() {
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
	testLoc := path.Join(here, "sites/test")
	if _, dirError := os.Stat(testLoc); !os.IsNotExist(dirError) {
		log.Println("site already exists. Let's get out")
		return
	}

	// create hugo site
	hugoPath, err := exec.LookPath("hugo")
	checkReason(err, "Couldn't find hugo. Make sure it's in your PATH")

	cmd := exec.Command(hugoPath, "new", "site", path.Join(sitesLoc, "test"))
	cmd.Dir = sitesLoc
	log.Printf("Creating new site in %s\n", cmd.Dir)
	err = cmd.Run()
	checkReason(err, "Error: couldn't create test site.")

	log.Println("Done setting up test site.")
}
