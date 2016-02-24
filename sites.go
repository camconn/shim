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
	"log"
	"os"
	"os/exec"
	"reflect"
)

var blankBytes = []byte{0}

// Site - Represent a Hugo site (as in blog.example.com)
type Site struct {
	location string
	title    string `desc:"the site's title"`
	baseurl  string `desc:"the baseurl of the site"`

	// The following directories are relative
	contentDir string `desc:"the content directory"`
	layoutDir  string `desc:"the layouts directory"`
	publishDir string `desc:"the publish directory"`

	builddrafts  bool `desc:"are drafts enabled?"`
	canonifyurls bool `desc:"are urls canonical?"`
	Posts        []*Post

	// TODO: Are these even needed?
	enabled bool // TODO: Implement Site.enabled
	public  bool // TODO: Implement Site.public
}

// Params - Parameters that the Hugo site uses globally in templates
type Params struct {
	description string `desc:"sort description of this site"`
	author      string `desc:"default author name for this site"`
}

func (s *Site) String() string {
	return fmt.Sprintf("Site \"%s\" - baseurl: \"%s\"", s.location, s.baseurl)
}

// SiteInfo - Print out basic information about this site
func (s *Site) SiteInfo() {
	fmt.Printf("Location: %s\n", s.location)

	// TODO: Bugfix related to blank site object
	st := reflect.TypeOf(Site{})

	fields := []string{
		"title",
		"baseurl",
		"contentDir",
		"layoutDir",
		"publishDir",
		"builddrafts",
		"canonifyurls",
	}

	// rt := reflect.TypeOf(s)
	for _, v := range fields {
		field, ok := st.FieldByName(v)
		if ok {
			strTag := field.Tag
			if true {
				// desc := field.Tag.Get("desc")
				// desc := strTag.Tag.Get("desc")
				desc := strTag.Get("desc")
				if len(desc) > 0 {
					fmt.Printf("%s: %s\n", v, desc)
				} else {
					fmt.Printf("%s\n", v)
				}
			} else {
				log.Fatal("fuck")
			}
		} else {
			log.Fatal("lolpls, apparently " + v + " isn't a valid field!")
		}
	}
}

// TODO: Don't implicitly rely on shimAssets being a global
func loadSite(dir, name string) *Site {
	s := Site{}

	viper.SetDefault("contentdir", "content")
	viper.SetDefault("layoutdir", "layouts")
	viper.SetDefault("publishdir", "public")
	viper.SetDefault("builddrafts", false)
	viper.SetDefault("baseurl", "http://yoursite.example.com/")
	viper.SetDefault("canonifyurls", true)
	viper.SetDefault("title", "My Hugo Site")

	// s.location = fmt.Sprintf("%s/%s/config.toml", dir, name)

	// Hugo_ROOT/sites/sitename
	s.location = fmt.Sprintf("%s/%s/%s", shimAssets.root, shimAssets.sites, name)

	file, err := os.Open(fmt.Sprintf("%s/config.toml", s.location))
	defer file.Close()
	check(err)
	viper.ReadConfig(file)

	s.baseurl = viper.GetString("baseurl")
	s.contentDir = viper.GetString("contentdir")
	s.layoutDir = viper.GetString("layoutdir")
	s.publishDir = viper.GetString("publishdir")
	s.builddrafts = viper.GetBool("builddrafts")
	s.canonifyurls = viper.GetBool("canonifyurls")
	s.title = viper.GetString("title")

	// TODO: Implement these
	s.public = true
	s.enabled = true

	return &s
}

// Build - Build and generate the site using Hugo's generator function
func (s Site) Build() error {
	hugoPath, err := exec.LookPath("hugo")
	if err != nil {
		return fmt.Errorf("Could not find hugo executable. Is hugo installed?\n")
	}

	publicDir := fmt.Sprintf("%s/public", s.location)
	cmd := exec.Command(hugoPath, "-s", s.location, "-d", publicDir)
	// log.Printf("command: %s\n", cmd)
	err = cmd.Run()
	if err != nil {
		log.Printf("WTF: %s\n", err.Error())
		return fmt.Errorf("Could not build site. Error: %s\n", err.Error())
	}
	return nil
}

// ContentDir - Return the directory of content
func (s Site) ContentDir() string {
	return s.contentDir
}
