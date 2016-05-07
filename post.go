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
	Location string
	RelPath  string
	Site     *Site

	Title       string
	author      string
	Description string // automatically generated
	ManualDesc  string // set by user
	Slug        string
	Draft       bool
	Published   *time.Time
	Aliases     []string
	Taxonomies  map[string][]string    // TODO: Is there a better storage format to use?
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
	p.Title = v.GetString("title")
	p.author = v.GetString("author")
	p.ManualDesc = v.GetString("description")
	p.Slug = v.GetString("slug")
	p.Draft = v.GetBool("draft")

	v.SetDefault("aliases", []string{})
	aliases := v.GetStringSlice("aliases")
	stripChars(&aliases, " ")
	removeDuplicates(&aliases)

	// Make sure we don't get any alias lists with only blank spaces
	if (len(aliases) > 1) || (len(aliases) == 1 && len(aliases[0]) > 0) {
		p.Aliases = aliases
	}

	{
		publishString := v.GetString("date")
		if len(publishString) > 0 {
			pTime, err := time.Parse(time.RFC3339, publishString)

			// Default to current time in case of parsing error
			if err != nil {
				pTime = time.Now()
			}

			p.Published = &pTime
		}
	}

	p.all = v.AllSettings()

	{
		// If the post is a draft, and there is no "editdate" key, then this post has
		// no date, even if we assigned it earlier.
		lastEditDate := v.GetString("editdate")
		if p.Draft && len(lastEditDate) == 0 {
			p.all["editdate"] = p.Date().Format(time.RFC3339)
			delete(p.all, "date")
			p.Published = nil
		}
	}

	for _, kind := range p.Site.Taxonomies().GetKinds() {
		plural := kind.Plural()
		terms := v.GetStringSlice(plural)
		if len(terms) != 0 {
			p.Taxonomies[plural] = terms
		}
	}
}

// contentDirPath is used to find the relative path of the post
func (s *Site) loadPost(postPath, contentDirPath string) (*Post, error) {
	var err error

	p := &Post{}
	p.Site = s
	p.Location, err = filepath.Abs(postPath)

	if err != nil {
		return nil, fmt.Errorf("Could not load post: invalid post path")
	}

	p.Taxonomies = make(map[string][]string)

	file, err := os.Open(postPath)
	defer file.Close()
	if err != nil {
		return nil, fmt.Errorf("Could not open post data file: %s\n", err.Error())
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
				return nil, fmt.Errorf("Couldn't load front matter for %s", postPath)
			}
		}
	}

	p.readTOMLMetadata(frontMatter)

	relativePathWithSuffix, err := filepath.Rel(contentDirPath, postPath)
	p.RelPath = strings.TrimSuffix(relativePathWithSuffix, filepath.Ext(filepath.Base(postPath)))

	_, err = file.Seek(pos, 0)
	if err != nil {
		return nil, fmt.Errorf("could not find post path: %s\n", err.Error())
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

		descriptionBytes = append(descriptionBytes, '\n', '\n', dot, dot, dot)
	}

	p.Description = (string)(descriptionBytes)

	return p, nil
}

// newPost creates a newPost in site/contentdir/NAME.md where NAME can be
// a relative directory which includes folder names. For example, the
// `name` argument could be "post/my-first-post.md". The returned value
// fPath is the absolute location of the post created if there is no error
// while creating the post. If the post already exists, fPath will be a blank
// string and an error will be returned.
func (s *Site) newPost(name string) (fPath string, err error) {
	testPostLoc := filepath.Join(s.Location, s.ContentDir(), name)
	if _, err = os.Stat(testPostLoc); !os.IsNotExist(err) {
		return "", fmt.Errorf("A page already exists at that location!")
	}

	hugoPath, err := exec.LookPath("hugo")
	if err != nil {
		return "", err
	}

	// TODO: Capture build output and send to logs
	cmd := exec.Command(hugoPath, "new", name)
	cmd.Dir = s.Location
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	fPath = path.Join(s.Location, s.ContentDir(), name)
	return
}

// SavePost - Save post to disk to path path
func (p *Post) SavePost(body string) error {
	// Go ahead and update the map of all TOML keys
	err := p.updateMap()
	if err != nil {
		return fmt.Errorf("Could not update post metadata: %s", err.Error())
	}

	// We're only writing here, we want to create a file if it doesn't exist,
	// and we want to truncate the file if we don't write the full thing.
	mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	file, err := os.OpenFile(p.Location, mode, 0666)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("Could not load post: %s\n", err.Error())
	}

	tomlEncoder := toml.NewEncoder(file)

	file.WriteString(tomlBoundary)
	tomlEncoder.Encode(p.all)
	file.WriteString(tomlBoundary)

	_, err = file.WriteString(body)
	if err != nil {
		return err
	}

	// TODO: Use a build queue or worker system
	go func() {
		var err error

		if p.Draft {
			err = p.Site.BuildPreview()
		} else {
			err = p.Site.BuildPublic()
		}
		if err != nil {
			log.Printf("Failed to run build in background: %s\n", err.Error())
		}
	}()

	go p.Site.loadTaxonomyTerms()
	return nil

}

// update the hashmap associated with this post
func (p *Post) updateMap() error {
	if p.all == nil {
		p.all = make(map[string]interface{})
	}

	p.all["author"] = p.Author()
	p.all["title"] = p.Title
	p.all["slug"] = p.Slug
	p.all["draft"] = p.Draft

	p.all["editdate"] = time.Now().Format(time.RFC3339)
	if p.HasDate() {
		p.all["date"] = p.Date().Format(time.RFC3339)
	} else {
		delete(p.all, "date")
	}

	if len(p.ManualDesc) > 0 {
		p.all["description"] = p.ManualDesc
	} else {
		delete(p.all, "description")
	}

	numAliases := len(p.Aliases)
	if numAliases > 1 || numAliases == 1 && len(p.Aliases[0]) != 0 {
		p.all["aliases"] = p.Aliases[:]
	} else {
		delete(p.all, "aliases")
	}

	err := p.updateTaxonomy()
	if err != nil {
		return fmt.Errorf("Could not update post taxonomies: %s\n", err.Error())
	}

	return nil
}

// update this site's view of this post's taxonomy
func (p *Post) updateTaxonomy() error {
	for term, values := range p.Taxonomies {

		// don't worry about tags which don't have anything
		if len(values) == 1 && len(values[0]) == 0 {
			continue
		}

		p.all[term] = values

		kind, err := p.Site.Taxonomies().GetTaxonomy(term)
		if err != nil {
			return fmt.Errorf("Tried to add a value to a taxonomy term that doesn't exist.")
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

	return nil
}

// TaxonomyMap get a hashmap of the taxonomies of a post followed by
// a joined string of each taxonomy's applicable values
func (p *Post) TaxonomyMap() map[string]string {
	items := make(map[string]string)
	for _, kind := range p.Site.Taxonomies().GetKinds() {
		term := kind.Plural()
		values := p.Taxonomies[term]
		items[term] = strings.Join(values, ", ")
	}

	return items
}

// Publish - Publish this post
func (p *Post) Publish(text string) error {
	p.Draft = false
	if p.Published == nil {
		p.Published = p.Date()
	}

	err := p.SavePost(text)
	if err != nil {
		return err
	}

	return nil
}

// Author is the writer of this post
func (p Post) Author() string {
	if len(p.author) == 0 {
		return p.Site.Author()
	}

	return p.author
}

// Date The published date of this post OR the last time it was edited
// if this post is a draft.
func (p Post) Date() *time.Time {
	if p.Published != nil {
		return p.Published
	}

	// If there's an edit date, try and use that for sorting.
	if p.Draft {
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
	return (p.Published != nil)
}

// WebDate - Get the date displayed in shim for this post
func (p Post) WebDate() string {
	return p.Date().Format(dateFormat)
}

func (p Post) String() string {
	return fmt.Sprintf("<Post title: %s; author: %s, date: %s; draft: %t>",
		p.Title, p.Author(), p.Date(), p.Draft)
}

// GetBody - Get the body of this post
func (p Post) GetBody() string {
	log.Printf("post location: %s\n", p.Location)
	pFile, err := os.Open(p.Location)
	if err != nil {
		return fmt.Sprintf("[ERROR]: Could not open post data file: %s", err.Error())
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
				return fmt.Sprintf("[ERROR]: Couldn't load text for this post: %s", p.Location)
			}
		} else if strings.Compare(line, tomlBoundary) == 0 {
			boundaryCount++
		}
	}

	return postBody.String()
}

// PostID is the base64 of the relative path for this post.
//
// See also: Post.RelPath
func (p *Post) PostID() string {
	relPathStr := p.RelPath

	b64Path := base64.StdEncoding.EncodeToString([]byte(relPathStr))

	return b64Path
}

// PreviewPath - Get the preview path for this post. This is effectively final
// path of the URL the page will be at after Hugo generates this page.
// TODO: Permalinks?
func (p Post) PreviewPath() string {
	if len(p.Slug) == 0 {
		return p.RelPath + "/"
	}

	return filepath.Join(p.RelPath, "..", p.Slug+"/")
}

// WebAliases - Access this post's aliases in a format for the web
func (p Post) WebAliases() string {
	return strings.Join(p.Aliases, ", ")
}
