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
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// WebWrapper - Struct for passing values to the web
type WebWrapper struct {
	Message string
	Action  string
	Site    *Site
	Post    *Post
	Status  bool
	Config  []configOption
	Text    *bytes.Buffer
	Choices []string
	URL     string
	// Even though these things are opposite, they imply different things
	Success bool
	Failed  bool
}

const (
	fiveMegabytes = (int64)(5 * 1024 * 1024)
)

// TODO: Just do it?
// var t = template.Must(template.ParseGlob("templates/*"))
// var t = template.Must(template.ParseGlob(fmt.Sprintf("%s/*", shimAssets.templates)))

func renderAnything(w http.ResponseWriter, tmpl string, i interface{}) {
	// TODO: Move this to a global variable
	t := template.Must(template.ParseGlob(fmt.Sprintf("%s/*", shimAssets.templates)))
	err := t.ExecuteTemplate(w, tmpl, i)
	if err != nil {
		log.Printf("Couldn't execute template: %s\n", err)
	}

}

// Home - The home page -- Just redirect to login
func Home(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "/admin/", http.StatusMovedPermanently)
}

// Admin - The admin page
func Admin(w http.ResponseWriter, req *http.Request) {
	status := new(WebWrapper)
	status.Action = ""
	status.Message = ""
	status.Success = false

	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			http.Error(w, "Couldn't parse form!", 500)
			return
		}

		req.ParseForm()

		doBuild := req.FormValue("doBuild")
		if doBuild == "1" {
			status.Action = "build"
		}
	}

	if status.Action == "build" {
		err := mySite.BuildPublic()
		if err != nil {
			status.Success = false
			status.Message = err.Error()
		} else {
			status.Success = true
		}
	}

	renderAnything(w, "adminPage", status)
}

// Login - The login page
func Login(w http.ResponseWriter, req *http.Request) {
	wrapper := new(WebWrapper)
	wrapper.Success = false

	q := req.URL.Query()
	wrapper.URL = req.URL.String()
	redirect := q.Get("redirect")
	if len(redirect) > 0 {

		warn := q.Get("warn")
		if len(warn) > 0 {
			wrapper.Message = "warn"
			q.Del("warn")
			wrapper.URL = "/login/?" + q.Encode()
		}
	}

	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			http.Error(w, "Couldn't parse form!", 500)
			return
		}

		username := req.FormValue("username")
		password := req.FormValue("password")

		cookie, success := um.LoginCookie(username, password, w, req)
		if success {
			fmt.Printf("Cookie: %s\n", cookie.String())
			http.SetCookie(w, cookie)

			if len(redirect) > 0 {
				log.Println("Redirecting to " + redirect)
				http.Redirect(w, req, redirect, http.StatusSeeOther)
				return
			}

			log.Println("Redirecting to /admin/.")
			http.Redirect(w, req, "/admin/", http.StatusSeeOther)
			return
		}

		log.Println("No success logging in")
		wrapper.Failed = true

		renderAnything(w, "loginPage", &wrapper)

	} else {
		renderAnything(w, "loginPage", &wrapper)
	}
}

// ViewPosts - View all posts
func ViewPosts(w http.ResponseWriter, req *http.Request) {
	mySite.GetAllPosts()

	renderAnything(w, "postsPage", &mySite)
}

// EditPost - Edit a Post
func EditPost(w http.ResponseWriter, req *http.Request) {
	wrapper := new(WebWrapper)
	wrapper.URL = req.URL.Path
	postPath := req.URL.Path[len("/edit/"):]

	if len(postPath) == 0 {
		http.Redirect(w, req, "/posts/", http.StatusTemporaryRedirect)
		return
	}

	contentDirPath := filepath.Join(mySite.Location(), mySite.ContentDir())
	postLoc := fmt.Sprintf("%s.md", filepath.Join(contentDirPath, postPath))
	fmt.Printf("location: %s\n", postLoc)
	post, err := loadPost(postLoc, contentDirPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)

		publish := false
		values := req.Form

		for i, v := range values {
			value := v[0]

			switch i {
			case "author":
				post.author = value
			case "doPublish":
				// Don't do anything, because we handled this earlier
				publish = true
			case "description":
				post.description = value
			case "published":
				// In reality, this doesn't matter for drafts because when a draft
				// is saved it's time published is set to when it was last modified.
				parsedTime, err := time.Parse(dateFormat, value)
				if err != nil {
					fmt.Printf("Couldn't parse time! %s\n", err.Error())
					wrapper.Failed = true
					wrapper.Message = "Could not parse publishing time! Please use a valid format, date, and time for time published."
					continue
				}
				post.published = &parsedTime
			case "slug":
				post.slug = value
			case "title":
				post.title = value
			case "articleSrc":
				post.body.Reset()
				post.body.WriteString(value)
			default:
				log.Printf("edit post ignoring %s and %s.\n", i, value)
			}
		}

		err = post.SavePost()
		if err != nil {
			log.Printf("Error while saving post: %s\n", err.Error())
			wrapper.Failed = true
			wrapper.Message = "Could not save post to disk. Error: " + err.Error()
			goto renderEditedPost
		} else if wrapper.Failed {
			// We try and save whatever data we do have if something failed before
			// this point (for example, if the time the user gave us was wrong, try
			// parsing it anyways, then try saving their post, then tell them that
			// an error occurred.
			goto renderEditedPost
		}

		if publish {
			err = post.Publish()
			if err != nil {
				log.Printf("Could not publish post: %s\n", err.Error())
				wrapper.Failed = true
				wrapper.Message = "Could not publish post: " + err.Error()
			}
		} else {
			post.draft = true
		}

		wrapper.Success = true
	}

renderEditedPost:
	wrapper.Post = post
	wrapper.Config = post.Options()

	renderAnything(w, "editPage", wrapper)
}

// NewPost - Create a new post
func NewPost(w http.ResponseWriter, req *http.Request) {
	wrapper := new(WebWrapper)
	wrapper.Success = false
	wrapper.Action = ""
	wrapper.Message = ""

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)
		newFilename := req.FormValue("title")

		if len(newFilename) > 0 {
			wrapper.Action = "build"

			contentDirPath := filepath.Join(mySite.Location(), mySite.ContentDir())
			testPostPath := filepath.Join(contentDirPath, "post", newFilename) + ".md"

			if _, err := os.Stat(testPostPath); !os.IsNotExist(err) {
				wrapper.Message = "Post already exists!"
				goto render
			}

			path, err := mySite.newPost(newFilename)
			if err != nil {
				wrapper.Message = "Could not edit post: " + err.Error()
				goto render
			}

			post, err := loadPost(path, contentDirPath)
			if err != nil {
				wrapper.Message = "Could not edit post: " + err.Error()
				goto render
			}

			post.draft = true
			err = post.SavePost()
			if err != nil {
				wrapper.Message = "Could not save post: " + err.Error()
				goto render
			}

			if err != nil {
				wrapper.Message = "Could not save post: " + err.Error()
				goto render
			} else {
				editLoc := fmt.Sprintf("/edit/%s", post.RelPath())
				log.Printf("redirecting to %s\n", editLoc)
				http.Redirect(w, req, editLoc, http.StatusTemporaryRedirect)
			}
		} else {
			wrapper.Message = "you need to specify a name!"
		}
	}

	// This is a failure point. Everything below this has to be safe.
render:
	renderAnything(w, "newPostPage", wrapper)

}

// RemovePost - Remove a Post
func RemovePost(w http.ResponseWriter, req *http.Request) {
	wrapper := new(WebWrapper)
	wrapper.URL = req.URL.String()

	relPath := req.URL.Path[len("/delete/"):]
	if len(relPath) == 0 {
		http.Error(w, "File not found :'(", 404)
	}

	fileLoc := filepath.Join(mySite.Location(), mySite.ContentDir(), relPath+".md")
	if _, err := os.Stat(fileLoc); os.IsNotExist(err) {
		http.Error(w, "File not found :'(", 404)
		return
	}

	contentDirPath := filepath.Join(mySite.Location(), mySite.ContentDir())
	post, err := loadPost(fileLoc, contentDirPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	wrapper.Post = post

	pageConfirmQuery := req.URL.Query()
	confirmation := pageConfirmQuery.Get("confirm")

	if confirmation == "yes" {
		log.Printf("Deleting %s\n", relPath)
		err := os.Remove(fileLoc)
		if err != nil {
			http.Error(w, "Couldn't delete file: "+err.Error(), 500)
		}
		http.Redirect(w, req, "/posts/", http.StatusTemporaryRedirect)
	}

	renderAnything(w, "deletePage", wrapper)
}

// EditSite - Edit a site's basic configuration
func EditSite(w http.ResponseWriter, req *http.Request) {
	// TODO: Support multiple sites
	wrapper := new(WebWrapper)
	wrapper.Site = mySite
	wrapper.Config = mySite.BasicConfig()
	wrapper.Success = false
	wrapper.Failed = false

	themesLoc := fmt.Sprintf("%s/%s", shimAssets.root, shimAssets.themes)
	allThemes, err := GetThemes(themesLoc)
	if err != nil {
		wrapper.Failed = true
		wrapper.Message = fmt.Sprintf("Failed to load themes: %s", err.Error())
		goto renderBasicConfig
	}
	wrapper.Choices = allThemes

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)

		// get values
		values := req.Form

		mySite.builddrafts = false
		mySite.canonifyurls = false

		for i, v := range values {
			//fmt.Printf("i: %s; v: %s\n", i, v)

			value := v[0]

			switch i {
			case "title":
				mySite.title = value
			case "baseurl":
				mySite.baseurl = value
			case "theme":
				mySite.theme = value
			case "contentDir":
				mySite.contentDir = value
			case "layoutDir":
				mySite.layoutDir = value
			case "publishDir":
				mySite.publishDir = value
			case "builddrafts":
				mySite.builddrafts = true
			case "canonifyurls":
				mySite.canonifyurls = true

				// Now for site-wide params
			case "params.author":
				mySite.author = value
			case "params.subtitle":
				mySite.subtitle = value
			default:
				log.Printf("WTF IS %s and %s?\n", i, value)
			}
		}

		// save site
		err := mySite.SaveConfig()
		if err != nil {
			wrapper.Failed = true
			wrapper.Message = fmt.Sprintf("Failed to save site: %s", err.Error())
			goto renderBasicConfig
		}

		err = ChangeTheme(mySite, mySite.Theme())
		if err != nil {
			wrapper.Failed = true
			wrapper.Message = fmt.Sprintf("Failed to change theme: %s", err.Error())
			goto renderBasicConfig
		}

		wrapper.Config = mySite.BasicConfig()

		wrapper.Success = true
	}
renderBasicConfig:
	renderAnything(w, "siteConfig", wrapper)
}

// AdvancedConfig - Edit a site's configuration (for power users)
func AdvancedConfig(w http.ResponseWriter, req *http.Request) {
	// TODO: Support multiple sites
	wrapper := new(WebWrapper)
	wrapper.Site = mySite
	wrapper.Success = false

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)

		// get values
		values := req.Form
		configSrc := values.Get("configSrc")

		// now write configSrc to file
		fileLoc := fmt.Sprintf("%s/config.toml", mySite.location)

		mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		file, err := os.OpenFile(fileLoc, mode, 0666)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		defer file.Close()

		// save site
		file.WriteString(configSrc)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		wrapper.Success = true
	}

	wrapper.Text = bytes.NewBuffer([]byte{})
	configLoc := fmt.Sprintf("%s/config.toml", mySite.location)
	file, err := os.Open(configLoc)
	defer file.Close()
	if err != nil {
		http.Error(w, "Can't open config file!", 500)
	}

	_, err = wrapper.Text.ReadFrom(file)
	if err != nil {
		http.Error(w, "Can't read config file!", 500)
	}

	renderAnything(w, "siteConfigAdvanced", wrapper)
}
