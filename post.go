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
	// dateFormat   = "2016-02-21T02:34:27-01:00"
)

// Post - Represents a post along with all its metadata
type Post struct {
	location    string
	title       string    `desc:"What the post is called"`
	author      string    `desc:"The person who wrote this post"`
	description string    `desc:"A short summary of this post"`
	slug        string    `desc:"The memorable URL of this post"`
	published   time.Time `desc:"When the post was published"`
	draft       bool      `desc:"Is this post a draft"`
	body        *bytes.Buffer
	all         map[string]interface{}
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
	p.description = v.GetString("description")
	p.slug = v.GetString("slug")
	p.draft = v.GetBool("draft")

	// Handle time parsing and error checking
	publishString := v.GetString("date")
	pTime, err := time.Parse(time.RFC3339, publishString)
	if err != nil {
		log.Fatalf("Error parsing time: %s\n", err)
	}
	p.published = pTime

	p.all = v.AllSettings()
}

func loadPost(path string) (p *Post, err error) {
	p = &Post{}
	p.location = path

	file, err := os.Open(path)
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
				log.Fatal("Couldn't load TOML front matter for ", path)
			}
		}
	}

	p.readTOMLMetadata(frontMatter)

	fStat, err := file.Stat()
	check(err)
	bodySize := fStat.Size() - pos - 1
	bodyReader := io.NewSectionReader(file, pos, bodySize)

	p.body = bytes.NewBuffer([]byte{})
	p.body.ReadFrom(bodyReader)

	// If the filename is still meh, then go ahead and change the slug
	if len(p.slug) == 0 {
		fNameWithSuffix := filepath.Base(path)
		fNameNoSuffix := strings.TrimSuffix(fNameWithSuffix, filepath.Ext(fNameWithSuffix))
		// fmt.Printf("Name with no suffix is %s\n", fNameNoSuffix)
		p.slug = fNameNoSuffix
	}

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
func (p Post) SavePost() error {

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

	// TODO: Be safer with writing to file

	// write TOML stuff
	tomlEncoder := toml.NewEncoder(file)

	file.WriteString(tomlBoundary)
	tomlEncoder.Encode(p.all)
	file.WriteString(tomlBoundary)

	_, err = p.body.WriteTo(file)
	if err != nil {
		return err
	}

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

	// TODO: Load everything else
}

// GetAllPosts - Find all posts in a folder
func (s *Site) GetAllPosts() {
	// TODO: Also load posts that are just in /content/ directory, and not any
	// of its subdirectories
	path := fmt.Sprintf("%s/%s", s.location, s.ContentDir())
	pattern := fmt.Sprintf("%s/*/*.md", path)
	fmt.Printf("Searching in %s\n", pattern)

	allNames, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatalf("There was an error while matching:\n%s\n", err.Error())
	}

	allPosts := make([]*Post, len(allNames))

	for i, v := range allNames {
		//log.Printf("Loading post from file %s\n", v)
		allPosts[i], err = loadPost(v)

		if err != nil {
			log.Printf("Was unable to load post %s: %s\n", v, err.Error())
		}
	}

	s.Posts = allPosts
}

// Publish - Publish this post
func (p *Post) Publish() error {
	p.draft = false
	p.published = time.Now()

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
	return &p.published
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
