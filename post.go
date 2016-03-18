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
	"encoding/base64"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
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
	description string // automatically generated
	manualDesc  string // set by user
	slug        string
	draft       bool
	published   *time.Time
	aliases     []string
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
	p.manualDesc = v.GetString("description")
	p.slug = v.GetString("slug")
	p.draft = v.GetBool("draft")

	v.SetDefault("aliases", []string{})
	aliases := v.GetStringSlice("aliases")
	stripChars(&aliases, " ")
	removeDuplicates(&aliases)
	// Make sure we don't get any alias lists with only blank spaces
	if (len(aliases) > 1) || (len(aliases) == 1 && len(aliases[0]) > 0) {
		p.aliases = aliases
	}

	{
		publishString := v.GetString("date")
		if publishString != "" {
			pTime, err := time.Parse(time.RFC3339, publishString)
			if err != nil {
				log.Fatalf("Error parsing time: %s\n", err)
			}
			p.published = &pTime
		}
	}

	p.all = v.AllSettings()

	{
		// If the post is a draft, and there is no "editdate" key, then this post has
		// no date, even if we assigned it earlier.
		lastEditDate := v.GetString("editdate")
		if p.Draft() && len(lastEditDate) == 0 {
			p.all["editdate"] = p.Date().Format(time.RFC3339)
			delete(p.all, "date")
			p.published = nil
		}
	}

	for _, kind := range p.Site().Taxonomies().GetKinds() {
		plural := kind.Plural()
		terms := v.GetStringSlice(plural)
		if len(terms) != 0 {
			p.taxonomies[plural] = terms
		}
	}
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
			if err != nil {
				log.Fatal("Couldn't load TOML front matter for ", postPath)
			}
		}
	}

	p.readTOMLMetadata(frontMatter)

	relativePathWithSuffix, err := filepath.Rel(contentDirPath, postPath)
	p.relpath = strings.TrimSuffix(relativePathWithSuffix, filepath.Ext(filepath.Base(postPath)))

	_, err = file.Seek(pos, 0)
	if err != nil {
		return nil, err
	}

	descriptionBuf := new(bytes.Buffer)
	descriptionBuf.ReadFrom(file)

	num := descriptionBuf.Len()
	if num > 160 {
		descriptionBuf.Truncate(160)
	}
	descriptionBytes := descriptionBuf.Bytes()

	descriptionBuf = nil

	const dot = '.'
	if num > 160 {

		// find last newline and truncate to make it pretty
		lastLineEnd := bytes.LastIndexByte(descriptionBytes, '\n')
		if lastLineEnd > 0 {
			descriptionBytes = descriptionBytes[:lastLineEnd]
		}

		descriptionBytes = append(descriptionBytes, '\n', '\n', '.', '.', '.')
	}

	p.description = (string)(descriptionBytes)

	return p, nil
}

// newPost creates a newPost in site/contentdir/NAME.md where NAME can be
// a relative directory which includes folder names. For example, the
// `name` argument could be "post/my-first-post.md". The returned value
// fPath is the absolute location of the post created if there is no error
// while creating the post. If the post already exists, fPath will be a blank
// string and an error will be returned.
func (s *Site) newPost(name string) (fPath string, err error) {
	testPostLoc := filepath.Join(s.Location(), s.ContentDir(), name)
	if _, err = os.Stat(testPostLoc); !os.IsNotExist(err) {
		return "", fmt.Errorf("A page already exists at that location!")
	}

	hugoPath, err := exec.LookPath("hugo")
	if err != nil {
		return "", err
	}

	// TODO: Capture build output and send to logs
	cmd := exec.Command(hugoPath, "new", name)
	cmd.Dir = s.location
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	fPath = path.Join(s.location, s.ContentDir(), name)
	return
}

// SavePost - Save post to disk to path path
func (p *Post) SavePost(body string) error {
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

	tomlEncoder := toml.NewEncoder(file)

	file.WriteString(tomlBoundary)
	tomlEncoder.Encode(p.all)
	file.WriteString(tomlBoundary)

	_, err = file.WriteString(body)
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
	p.all["draft"] = p.Draft()

	p.all["editdate"] = time.Now().Format(time.RFC3339)
	if p.HasDate() {
		p.all["date"] = p.Date().Format(time.RFC3339)
	} else {
		delete(p.all, "date")
	}

	if len(p.ActualDescription()) > 0 {
		p.all["description"] = p.ActualDescription()
	} else {
		delete(p.all, "description")
	}

	numAliases := len(p.aliases)
	if numAliases > 1 || numAliases == 1 && len(p.aliases[0]) != 0 {
		p.all["aliases"] = p.aliases[:]
	} else {
		delete(p.all, "aliases")
	}

	p.updateTaxonomy()
}

// update this site's view of this post's taxonomy
func (p *Post) updateTaxonomy() {
	for term, values := range (*p).taxonomies {

		// don't worry about tags which don't have anything
		if len(values) == 1 && len(values[0]) == 0 {
			continue
		}

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
func (p *Post) Publish(text string) error {
	p.draft = false
	if p.published == nil {
		p.published = p.Date()
	}

	err := p.SavePost(text)
	if err != nil {
		return err
	}

	return nil
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

// Date The published date of this post OR the last time it was edited
// if this post is a draft.
func (p Post) Date() *time.Time {
	if p.published != nil {
		return p.published
	}

	// If there's an edit date, try and use that for sorting.
	if p.Draft() {
		if editTimeValue, ok := p.all["editdate"]; ok {
			if editTimeStr, ok := editTimeValue.(string); ok {
				editTime, err := time.Parse(time.RFC3339, editTimeStr)
				if err == nil {
					return &editTime
				}
				log.Println("Couldn't parse time: " + err.Error())
			}
		}
	}

	now := time.Now()
	return &now
}

// HasDate lets you know if the user has manually specified a date for this post
func (p Post) HasDate() bool {
	return (p.published != nil)
}

// WebDate - Get the date displayed in shim for this post
func (p Post) WebDate() string {
	return p.Date().Format(dateFormat)
}

// Draft - Is this post a draft?
func (p Post) Draft() bool {
	return p.draft
}

// ActualDescription - Get a short description of this post as a raw string,
// without calculating it on the fly
func (p Post) ActualDescription() string {
	return p.manualDesc
}

// Description - A short description of this post.
func (p Post) Description() string {
	return p.description
}

func (p *Post) String() string {
	return fmt.Sprintf("<Post title: %s; author: %s, date: %s; draft: %t>",
		p.Title(), p.Author(), p.Date(), p.Draft())
}

// GetBody - Get the body of this post
func (p Post) GetBody() string {
	log.Printf("post location: %s\n", p.location)
	pFile, err := os.Open(p.location)
	if err != nil {
		return ""
	}

	postBody := bytes.NewBuffer([]byte{})
	frontScanner := bufio.NewScanner(pFile)

	pos := (int64)(0)
	boundaryCount := 0

	for frontScanner.Scan() {
		line := fmt.Sprintf("%s\n", frontScanner.Text())
		pos += (int64)(len(line))
		// TODO: Also support YAML
		if boundaryCount == 2 {
			_, err := postBody.WriteString(line)
			if err != nil {
				log.Fatalf("Couldn't load body for %s\n", p.location)
			}
		} else if strings.Compare(line, tomlBoundary) == 0 {
			boundaryCount++
		}
	}

	return postBody.String()
}

// RelPath - Get the relative path of this post to the /content/ directory
func (p Post) RelPath() string {
	return p.relpath
}

// PostID is the base64 of the relative path for this post.
//
// See also: Post.RelPath
func (p *Post) PostID() string {
	relPathStr := p.RelPath()

	b64Path := base64.StdEncoding.EncodeToString([]byte(relPathStr))

	return b64Path
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

// WebAliases - Access this post's aliases in a format for the web
func (p Post) WebAliases() string {
	return strings.Join(p.aliases, ", ")
}
