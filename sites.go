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
	"container/list"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
)

// SitePosts - An array of pointers to all of this site's posts
type SitePosts []*Post

// Site - Represent a Hugo site (as in blog.example.com)
type Site struct {
	location  string
	shortName string
	title     string
	subtitle  string
	baseurl   string
	author    string
	theme     string

	// The following directories are relative
	contentDir string
	layoutDir  string
	publishDir string

	builddrafts  bool
	canonifyurls bool
	Posts        SitePosts

	// Don't prefer to modify this map directly! Instead, prefer to modify the fields
	// of this Site struct, as they *overwrite* this hashmap!
	allSettings map[string]interface{}
	taxonomies  TaxonomyKinds

	buildLock struct {
		lock *sync.Mutex
	}
}

func (s *Site) String() string {
	return fmt.Sprintf("Site \"%s\" - baseurl: \"%s\"", s.location, s.baseurl)
}

// Sorting stuff used to order posts on the view posts page.
// Posts are sorted in the order such that drafts come first, then come
// older posts in ascending order.
func (s SitePosts) Len() int      { return len(s) }
func (s SitePosts) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s SitePosts) Less(i, j int) bool {
	postA := s[i]
	postB := s[j]
	if postA.Draft() && !postB.Draft() {
		return true
	}

	timeA := postA.Date().Add(0)
	timeB := postB.Date().Add(0)
	return !timeA.Before(timeB)
}

// Reload - Reload this site from configuration
func (s Site) Reload() {
	s.loadConfig(s.ShortName())
}

func loadSite(name string) Site {
	s := Site{}
	(&s).loadConfig(name)
	s.buildLock.lock = &sync.Mutex{}
	return s
}

func (s *Site) loadConfig(name string) {
	// This is (Hugo_ROOT/sites/sitename)
	s.location = filepath.Join(shimAssets.root, shimAssets.sites, name)
	s.shortName = name

	confLoc := fmt.Sprintf("%s/config.toml", s.location)
	fmt.Printf("Opening config at %s\n", confLoc)
	file, err := os.Open(confLoc)
	defer file.Close()
	check(err)

	v := viper.New()
	v.SetConfigType("toml")
	err = v.ReadConfig(file)
	checkReason(err, "Can't read config")

	v.SetDefault("contentdir", "content")
	v.SetDefault("layoutdir", "layouts")
	v.SetDefault("publishdir", "public")
	v.SetDefault("builddrafts", false)
	v.SetDefault("baseurl", "http://myblog.example.com/")
	v.SetDefault("canonifyurls", false)
	v.SetDefault("title", "My Hugo+Shim Site")
	v.SetDefault("theme", "slim")

	// Struct-builtin fields that are in `params.NAME`
	defaultParams := make(map[string]interface{})
	defaultParams["author"] = "John Doe"
	defaultParams["subtitle"] = "My Shim Blog"
	v.SetDefault("params", defaultParams)

	s.baseurl = v.GetString("baseurl")
	s.contentDir = v.GetString("contentdir")
	s.layoutDir = v.GetString("layoutdir")
	s.publishDir = v.GetString("publishdir")
	s.builddrafts = v.GetBool("builddrafts")
	s.canonifyurls = v.GetBool("canonifyurls")
	s.title = v.GetString("title")
	s.theme = v.GetString("theme")
	s.subtitle = v.GetString("params.subtitle")
	s.author = v.GetString("params.author")

	// Set sane defaults for taxonomies
	taxDefaults := map[string]string{
		"tag":      "tags",
		"category": "categories",
	}
	v.SetDefault("taxonomies", taxDefaults)

	s.taxonomies = make(TaxonomyKinds)
	taxonomies := v.GetStringMapString("taxonomies")

	if taxonomies != nil {
		for singular, plural := range taxonomies {
			s.taxonomies.NewTaxonomy(singular, plural)
		}
	}

	s.loadTaxonomyTerms()

	s.allSettings = v.AllSettings()
}

// From all posts, populate each taxonomy with terms
func (s Site) loadTaxonomyTerms() {
	// clear existing terms if reloading
	for _, kind := range s.Taxonomies() {
		kind.Clear()
	}
	// Update taxonomy from each post
	s.Posts = nil
	s.GetAllPosts()
	for _, p := range s.Posts {
		p.updateTaxonomy()
	}
}

// GetAllPosts - Find all posts for this site.
// TODO: Don't reload posts if they haven't been modified since last load.
func (s *Site) GetAllPosts() {
	contentPath := filepath.Join(s.Location(), s.ContentDir())

	allPostFiles := list.New()
	numPosts := 0

	scanFunc := func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			// return err
		}

		if !fileInfo.IsDir() {
			ext := filepath.Ext(path)
			// for now, we only care about Markdown files
			if ext == ".md" {
				(*allPostFiles).PushBack(path)
				numPosts++
			}
		}
		return nil
	}

	err := filepath.Walk(contentPath, scanFunc)
	check(err)

	allPosts := make([]*Post, numPosts)

	elem := allPostFiles.Front()

	for i := 0; i < numPosts; i++ {
		nameValue := elem.Value
		fileName, ok := nameValue.(string)
		if ok {
			allPosts[i], err = s.loadPost(fileName, contentPath)
			if err != nil {
				log.Fatalf("failed to load post %s!\n", fileName)
			}
		} else {
			log.Fatal("Failed horribly while walking through file path")
		}
		elem = elem.Next()

	}

	s.Posts = allPosts
	sort.Sort(s.Posts)
}

// BuildPublic - Build the public site using Hugo
func (s *Site) BuildPublic() (err error) {
	publicDir := filepath.Join(s.Location(), "public")
	err = s.build(publicDir, false)
	return
}

// BuildPreview - Build a preview with hugo
func (s Site) BuildPreview() (err error) {
	// Set baseurl to /preview/ to help view
	origPath := s.BaseURL()[:]

	// NOTE: The way that we are publishing these sites means that all of the URLs
	// **CANNOT** be canonical. So what's happening here is that we're temporarily
	// making the site not canonical for the entirety of this build process.
	wasCanonical := s.Canonify()
	if wasCanonical {
		s.canonifyurls = false
	}

	s.baseurl = shimAssets.baseurl + "/preview/"
	log.Printf("Temporarily %s\n", s.BaseURL())
	// s.baseurl = origPath + "/preview/"
	err = s.SaveConfig()
	if err != nil {
		return
	}

	previewDir := filepath.Join(s.Location(), "preview")
	err = s.build(previewDir, true)
	if err != nil {
		return
	}

	// reset to original path
	s.baseurl = origPath
	s.canonifyurls = wasCanonical
	_ = s.SaveConfig()
	log.Printf("Now %s\n", s.BaseURL())

	return err
}

// Build and generate the site using Hugo's generator function
func (s *Site) build(path string, drafts bool) error {
	hugoPath, err := exec.LookPath("hugo")
	if err != nil {
		return fmt.Errorf("Could not find hugo executable. Is hugo installed?\n")
	}

	// These two lines so we don't screw ourselves accidentally with Hugo.
	s.buildLock.lock.Lock()
	defer s.buildLock.lock.Unlock()

	// go ahead and clean up the current public directory
	err = os.RemoveAll(path)
	if err != nil {
		log.Fatalf("Could not clean up target %s\nError: %s", path, err.Error())
	}

	cmd := &exec.Cmd{}
	if drafts {
		cmd = exec.Command(hugoPath, "-D", "-s", s.Location(), "-d", path)
	} else {
		cmd = exec.Command(hugoPath, "-s", s.Location(), "-d", path)
	}
	err = cmd.Run()
	if err != nil {
		log.Println("WTF: " + err.Error())
		return fmt.Errorf("Could not build site. Error: %s\n", err.Error())
	}

	return nil
}

// SaveConfig - Saves this site's configuration with the intended changes
func (s Site) SaveConfig() error {
	s.updateMap()

	// fmt.Printf("opening at %s/config.toml\n", s.location)
	fileLoc := fmt.Sprintf("%s/config.toml", s.location)

	mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	file, err := os.OpenFile(fileLoc, mode, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("# WARNING: This file was automatically generated by shim.\n" +
		"# Editing this file directly may have adverse consequences.\n" +
		"# Even though TOML is CaSe-SeNsItIvE, viper, Hugo's TOML parser is case-insensitive!\n" +
		"# Thus, all of the key values below are lowercase to prevent duplication.\n\n" +
		"# You have been warned!\n\n")
	checkReason(err, "Could not write warning header to top of site configuration "+
		"file. It may be corrupted, so please check it.")

	tomlEncoder := toml.NewEncoder(file)
	tomlEncoder.Indent = "    "
	tomlEncoder.Encode(s.allSettings)

	return nil
}

// internal method called by SaveConfig
func (s *Site) updateMap() {
	if s.AllSettings() == nil {
		fmt.Println("Writing a new map in updateMap(). This is scary!")
		s.allSettings = make(map[string]interface{})
	}

	// Even though TOML is case-sensitive, viper, the library Hugo uses, is not.
	// Therefore, everything is lowercase here to prevent accidental duplication.
	s.allSettings["title"] = s.Title()
	s.allSettings["baseurl"] = s.BaseURL()
	s.allSettings["contentdir"] = s.ContentDir()
	s.allSettings["layoutdir"] = s.LayoutDir()
	s.allSettings["publishdir"] = s.PublishDir()
	s.allSettings["staticdir"] = "static"
	s.allSettings["builddrafts"] = s.BuildDrafts()
	s.allSettings["canonifyurls"] = s.Canonify()
	s.allSettings["theme"] = s.Theme()
	s.allSettings["metadataformat"] = "toml"
	s.allSettings["notimes"] = false
	s.allSettings["paginate"] = 10
	s.allSettings["paginatepath"] = "page"

	paramsKey, ok := s.allSettings["params"]
	if ok {
		paramsMap, ok := paramsKey.(map[string]interface{})
		if ok {
			// Site-wide parameters
			paramsMap["author"] = s.Author()
			paramsMap["subtitle"] = s.Subtitle()
		} else {
			log.Fatal("allSettings[\"params\"] is *not* a map[string]interface{}! WTF?!!?!?!")
		}
	} else {
		log.Fatal("Could not find \"params\" map inside of configuration map.")
	}

	// Go ahead and write all taxonomy names to the hashmap
	taxonomies := make(map[string]string)
	for _, kind := range s.taxonomies.GetKinds() {
		taxonomies[kind.Singular()] = kind.Plural()
	}
	s.allSettings["taxonomies"] = taxonomies
}

/* These are just accessors */

// Title - Get title of site
func (s Site) Title() string {
	return s.title
}

// Subtitle - Get title of site
func (s Site) Subtitle() string {
	return s.subtitle
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

// AllSettings - Get a map of all settings for this site
func (s Site) AllSettings() map[string]interface{} {
	return s.allSettings
}

// Author - Default author for the site
func (s Site) Author() string {
	return s.author
}

// Theme - This site's theme
func (s Site) Theme() string {
	return s.theme
}

// ShortName - This site's short name
func (s Site) ShortName() string {
	return s.shortName
}

// Location - This site's location on disk
func (s Site) Location() string {
	return s.location
}

// Taxonomies - This site's taxonomies
func (s Site) Taxonomies() TaxonomyKinds {
	return s.taxonomies
}
