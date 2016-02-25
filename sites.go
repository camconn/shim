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
	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
	"reflect"
)

var blankBytes = []byte{0}

// Site - Represent a Hugo site (as in blog.example.com)
type Site struct {
	location  string
	shortName string
	title     string `desc:"the site's title"`
	subtitle  string `desc:"the site's subtitle"`
	baseurl   string `desc:"the baseurl of the site"`

	// The following directories are relative
	contentDir string `desc:"the content directory"`
	layoutDir  string `desc:"the layouts directory"`
	publishDir string `desc:"the publish directory"`

	builddrafts  bool `desc:"are drafts enabled?"`
	canonifyurls bool `desc:"are urls canonical?"`
	Posts        []*Post

	// Map of *all* settings. Used to modify site settings.
	// Don't modify these directly, if possible. If you do modify these,
	// make sure the appropriate fields in this Site are also set.
	allSettings map[string]interface{}

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
func (s Site) SiteInfo() []configOption {
	// fmt.Printf("Location: %s\n", s.location)

	// TODO: Bugfix related to blank site object

	fields := []string{
		"title",
		"subtitle",
		"baseurl",
		"contentDir",
		"layoutDir",
		"publishDir",
		"builddrafts",
		"canonifyurls",
	}

	stv := reflect.ValueOf(s)
	stt := stv.Type()
	items := make([]configOption, len(fields))

	for i := 0; i < len(fields); i++ {
		help := configOption{}
		name := fields[i]

		help.Name = name
		// fmt.Printf("looking for %s\n", name)

		field := stv.FieldByName(name)
		// fmt.Printf("okay, now what?!")
		// fmt.Println(field)
		strField, ok := stt.FieldByName(name)

		if ok {
			// TODO: Is this bad?
			help.Description = strField.Tag.Get("desc")
		} else {
			help.Description = ""
		}

		// decide which interface to use
		switch field.Kind() {
		case reflect.Bool:
			help.Value = field.Bool()
			help.Type = "bool"
		case reflect.String:
			help.Value = field.String()
			help.Type = "string"
		case reflect.Int:
			help.Value = field.Int()
			help.Type = "int"
		case reflect.Float32:
		case reflect.Float64:
			help.Value = field.Float()
			help.Type = "float"
		default:
			fmt.Printf("I don't know what reflect.Kind this is! %##v\n", field.Kind())
		}

		// fmt.Printf("configOption struct: %##v\n", help)
		items[i] = help
	}

	return items
}

// TODO: Don't implicitly rely on shimAssets being a global
func loadSite(dir, name string) *Site {
	s := Site{}

	v := viper.New()
	v.SetDefault("contentdir", "content")
	v.SetDefault("layoutdir", "layouts")
	v.SetDefault("publishdir", "public")
	v.SetDefault("builddrafts", false)
	v.SetDefault("baseurl", "http://yoursite.example.com/")
	v.SetDefault("canonifyurls", true)
	v.SetDefault("title", "My Hugo Site")
	v.SetDefault("subtitle", "")

	// This is (Hugo_ROOT/sites/sitename)
	s.location = fmt.Sprintf("%s/%s/%s", shimAssets.root, shimAssets.sites, name)
	s.shortName = name

	file, err := os.Open(fmt.Sprintf("%s/config.toml", s.location))
	defer file.Close()
	check(err)
	v.ReadConfig(file)

	s.baseurl = v.GetString("baseurl")
	s.contentDir = v.GetString("contentdir")
	s.layoutDir = v.GetString("layoutdir")
	s.publishDir = v.GetString("publishdir")
	s.builddrafts = v.GetBool("builddrafts")
	s.canonifyurls = v.GetBool("canonifyurls")
	s.title = v.GetString("title")
	s.subtitle = v.GetString("subtitle")

	s.allSettings = v.AllSettings()

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

// SaveConfig - Saves this site's configuration with the intended changes
func (s Site) SaveConfig() error {
	s.updateMap()

	fileLoc := fmt.Sprintf("%s/config.toml", s.location)
	file, err := os.Open(fileLoc)
	if err != nil {
		return err
	}
	defer file.Close()

	// write TOML stuff
	tomlEncoder := toml.NewEncoder(file)
	tomlEncoder.Encode(s.allSettings)

	return nil
}

// internal method called by SaveConfig
func (s *Site) updateMap() {
	if s.allSettings == nil {
		s.allSettings = make(map[string]interface{})
	}

	s.allSettings["title"] = s.Title()
	s.allSettings["baseurl"] = s.BaseURL()
	s.allSettings["contentdir"] = s.ContentDir()
	s.allSettings["layoutdir"] = s.LayoutDir()
	s.allSettings["publishdir"] = s.PublishDir()
	s.allSettings["builddrafts"] = s.BuildDrafts()
	s.allSettings["canonifyurls"] = s.Canonify()

	// TODO: Load everything else
}

/* These are just accessors */

// Title - Get title of site
func (s Site) Title() string {
	return s.title
}

// Subtitle - Get title of site
func (s Site) Subtitle() string {
	return s.title
}

// BaseURL - Get base url of site
func (s Site) BaseURL() string {
	return s.baseurl
}

// ContentDir - Directory where content (e.g. posts, images) will be stored
func (s Site) ContentDir() string {
	return s.contentDir
}

// LayoutDir - Directory containing layout tempates
func (s Site) LayoutDir() string {
	return s.layoutDir
}

// PublishDir - Where to publish the built site to
func (s Site) PublishDir() string {
	return s.publishDir
}

// BuildDrafts - Return the directory of content
func (s Site) BuildDrafts() bool {
	return s.builddrafts
}

// Canonify - Do we canonify URLs?
func (s Site) Canonify() bool {
	return s.canonifyurls
}
