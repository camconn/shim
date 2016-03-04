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
	"bufio"
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	tomlBoundary = "+++\n"
	dateFormat   = "_2 Jan 2006 @ 15:04"
)

// Post - Represents a post along with all its metadata
type Post struct {
	// These are never edited by us. They are effectively constants.
	location string
	relpath  string
	site     *Site

	title       string
	author      string
	description string
	slug        string
	draft       bool
	published   *time.Time
	body        *bytes.Buffer
	taxonomies  map[string][]string    // TODO: Is there a better storage format to use?
	all         map[string]interface{} // All TOML data for this file
}

// Read the TOML metadata from a byte array and return a hashmap
func (p *Post) readTOMLMetadata(data io.Reader) {
	v := viper.New()
	v.SetConfigType("toml")
	v.ReadConfig(data)

	p.handleFrontMatter(v)
}

func (p *Post) handleFrontMatter(v *viper.Viper) {
	p.title = v.GetString("title")
	p.author = v.GetString("author")
	if len(p.Author()) == 0 {
		p.author = p.Site().Author()
	}
	p.description = v.GetString("description")
	p.slug = v.GetString("slug")
	p.draft = v.GetBool("draft")

	// Handle time parsing and error checking
	publishString := v.GetString("date")
	if publishString != "" {
		pTime, err := time.Parse(time.RFC3339, publishString)
		if err != nil {
			log.Fatalf("Error parsing time: %s\n", err)
		}
		p.published = &pTime
	}

	for _, kind := range p.Site().Taxonomies().GetKinds() {
		plural := kind.Plural()
		terms := v.GetStringSlice(plural)
		if len(terms) != 0 {
			p.taxonomies[plural] = terms
		}
	}

	p.all = v.AllSettings()
}

// contentDirPath is used to find the relative path of the post
func (s *Site) loadPost(postPath, contentDirPath string) (p *Post, err error) {
	p = &Post{}
	p.site = s
	p.location, err = filepath.Abs(postPath)
	p.taxonomies = make(map[string][]string)
	check(err)

	file, err := os.Open(postPath)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	frontMatter := bytes.NewBuffer([]byte{})
	frontScanner := bufio.NewScanner(file)
	pos := (int64)(0)
	boundaryCount := 0

	for frontScanner.Scan() && (boundaryCount < 2) {
		line := fmt.Sprintf("%s\n", frontScanner.Text())
		pos += (int64)(len(line))
		// TODO: Also support YAML
		if strings.Compare(line, tomlBoundary) == 0 {
			boundaryCount++
		} else {
			_, err := frontMatter.WriteString(line)
			// fmt.Printf("%d lines written\n", n)
			if err != nil {
				log.Fatal("Couldn't load TOML front matter for ", postPath)
			}
		}
	}

	p.readTOMLMetadata(frontMatter)

	fStat, err := file.Stat()
	check(err)
	bodySize := fStat.Size() - pos
	bodyReader := io.NewSectionReader(file, pos, bodySize)

	p.body = bytes.NewBuffer([]byte{})
	p.body.ReadFrom(bodyReader)

	relativePathWithSuffix, err := filepath.Rel(contentDirPath, postPath)
	p.relpath = strings.TrimSuffix(relativePathWithSuffix, filepath.Ext(filepath.Base(postPath)))

	return p, nil
}

// hugo new post/`name`.md
func (s *Site) newPost(name string) (path string, err error) {
	// TODO: Check if post already exists

	hugoPath, err := exec.LookPath("hugo")
	check(err)

	cmd := exec.Command(hugoPath, "new", fmt.Sprintf("post/%s.md", name))
	cmd.Dir = s.location
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	path = fmt.Sprintf("%s/%s/post/%s.md", s.location, s.ContentDir(), name)

	return
}

// SavePost - Save post to disk to path path
func (p *Post) SavePost() error {
	if p.Draft() { // If saving a draft, the time updated is right now
		now := time.Now()
		p.published = &now
	}

	// Go ahead and update the map of all TOML keys
	p.updateMap()

	// We're only writing here, we want to create a file if it doesn't exist,
	// and we want to truncate the file if we don't write the full thing.
	mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	file, err := os.OpenFile(p.location, mode, 0666)
	defer file.Close()
	if err != nil {
		return err
	}

	// write TOML stuff
	tomlEncoder := toml.NewEncoder(file)

	file.WriteString(tomlBoundary)
	tomlEncoder.Encode(p.all)
	file.WriteString(tomlBoundary)

	_, err = file.WriteString(p.body.String())
	if err != nil {
		return err
	}

	// Go ahead and rebuild the site
	go func() {
		err = nil
		if p.Draft() {
			err = p.Site().BuildPreview()
		} else {
			err = p.Site().BuildPublic()
		}
		if err != nil {
			log.Printf("Failed to run build in background: %s\n", err.Error())
		}
	}()

	go p.Site().loadTaxonomyTerms()
	return nil

}

// update the hashmap associated with this post
func (p *Post) updateMap() {
	if p.all == nil {
		p.all = make(map[string]interface{})
	}

	p.all["author"] = p.Author()
	p.all["title"] = p.Title()
	p.all["slug"] = p.Slug()
	p.all["date"] = p.Date()
	p.all["draft"] = p.Draft()
	p.all["description"] = p.Description()

	p.updateTaxonomy()
}

// update this site's view of this post's taxonomy
func (p *Post) updateTaxonomy() {
	for term, values := range (*p).taxonomies {
		(*p).all[term] = values

		kind, err := p.Site().Taxonomies().GetTaxonomy(term)
		if err != nil {
			log.Fatal("Tried to add a value to a taxonomy term that doesn't exist.")
		}

		for _, v := range values {
			v = strings.TrimSpace(v)
			if len(v) > 0 {
				_, err = kind.GetTerm(v)
				// if term doesn't already exist, add it
				if err != nil {
					kind.AddTerm(v)
				}
			}
		}
	}
}

// TaxonomyMap get a hashmap of the taxonomies of a post followed by the
// a joined string of each taxonomy's applicable values
func (p *Post) TaxonomyMap() map[string]string {
	items := make(map[string]string)
	for _, kind := range p.Site().Taxonomies().GetKinds() {
		term := kind.Plural()
		values := (*p).taxonomies[term]
		items[term] = strings.Join(values, ", ")
	}

	return items
}

// Publish - Publish this post
func (p *Post) Publish() error {
	if p.published == nil {
		now := time.Now()
		p.published = &now
	}
	p.draft = false

	err := p.SavePost()
	if err != nil {
		return err
	}

	return nil
}

// ClearSettings - clear the hashmap of the full list of settings for this post
func (p *Post) ClearSettings() {
	p.all = nil
}

// Author - The author of this post
func (p Post) Author() string {
	return p.author
}

// Title - The title of this post
func (p Post) Title() string {
	return p.title
}

// Slug - The short URL of this post
func (p Post) Slug() string {
	return p.slug
}

// Date - The published date of this post
func (p Post) Date() *time.Time {
	if p.published == nil {
		now := time.Now()
		return &now
	}
	return p.published
}

// Draft - Is this post a draft?
func (p Post) Draft() bool {
	return p.draft
}

// Description - A short description of this post.
func (p Post) Description() string {
	return p.description
}

func (p *Post) String() string {
	//return fmt.Sprintf("<Post title: %s; author: %s, date: %s; draft: %t>",
	//	p.Title(), p.Author(), p.Date(), p.Draft())
	return fmt.Sprintf("<Post title: %s; author: %s, date: %s; draft: %t>",
		p.Title(), p.Author(), p.Date(), p.Draft())
}

// GetBody - Get the body of this post
func (p Post) GetBody() []byte {
	return p.body.Bytes()
}

// RelPath - Get the relative path of this post to the /content/ directory
func (p Post) RelPath() string {
	return p.relpath
}

// WebDate - Get the date displayed in shim for this post
func (p Post) WebDate() string {
	return p.Date().Format(dateFormat)
}

// PreviewPath - Get the preview path for this post. This is effectively final
// path of the URL the page will be at after Hugo generates this page.
// TODO: Permalinks?
func (p Post) PreviewPath() string {
	if len(p.Slug()) == 0 {
		return p.RelPath() + "/"
	}

	return filepath.Join(p.RelPath(), "..", p.Slug()) + "/"
}

// Site - This post's site
func (p Post) Site() *Site {
	return p.site
}
