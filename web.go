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
	"path"
	"strings"
)

// TODO: Create a generic wrapper and use that to feed pages info.
type loginStatus struct {
	Failed bool
}

type siteStatus struct {
	Success bool
	Build   bool
	Message string
}

// WebWrapper - Struct for passing values to the web
// TODO: Use this instead of other structs
type WebWrapper struct {
	Message string
	Action  string
	Site    *Site
	Post    *Post
	Status  bool
	Success bool
	Config  []configOption
	Text    *bytes.Buffer
}

const (
	fiveMegabytes = (int64)(5 * 1024 * 1024)
)

// TODO: Just do it?
// var t = template.Must(template.ParseGlob("templates/*"))
// var t = template.Must(template.ParseGlob(fmt.Sprintf("%s/*", shimAssets.templates)))

// TODO: These templates aren't reloaded dynamically. Let's load them dynamically.
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
	log.Println("got a hit on home")

	http.Redirect(w, req, "/login/", http.StatusMovedPermanently)
}

// Admin - The admin page
func Admin(w http.ResponseWriter, req *http.Request) {
	// TODO: Require Login

	status := new(siteStatus)
	status.Build = false
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
			status.Build = true
		}
	}

	if status.Build {
		err := mySite.Build()
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
	status := new(loginStatus)
	status.Failed = false
	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			http.Error(w, "Couldn't parse form!", 500)
			return
		}

		username := req.FormValue("username")
		password := req.FormValue("password")

		log.Printf("User tried to login with \"%s\" and \"%s\"\n", username, password)

		status.Failed = true

		renderAnything(w, "loginPage", &status)

	} else {
		renderAnything(w, "loginPage", &status)
	}
}

// ViewPosts - View all posts
func ViewPosts(w http.ResponseWriter, req *http.Request) {
	// TODO: Require login
	log.Println("Got a hit on a post!")

	mySite.GetAllPosts()

	renderAnything(w, "postsPage", &mySite)
}

// EditPost - Edit a Post
func EditPost(w http.ResponseWriter, req *http.Request) {
	// TODO: Require login
	log.Println("Got a hit on a edit!")

	var post *Post
	post = nil

	// TODO: Be able to create a new post
	// TODO: Randomize and handle zero-length ids
	// TODO: Support loading posts from anywhere in site/content/
	id := req.URL.Path[len("/edit/"):]
	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)
		log.Println("Got POST")

		if len(id) == 0 {
			id = "Untitled"
		}

		postLoc := fmt.Sprintf("%s/%s/post/%s.md", mySite.location, mySite.ContentDir(), id)
		fmt.Printf("location: %s\n", postLoc)
		post, err := loadPost(postLoc)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		title := req.FormValue("title")
		body := req.FormValue("articleSrc")

		post.title = title
		post.body.Reset()
		post.body.WriteString(body)

		publish := req.FormValue("doPublish")
		log.Printf("doPublish: %s\n", publish)

		post.SavePost()
		// TODO: handle site build errors more gracefully
		err = mySite.Build()
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	}

	// TODO: Show an editable list of posts if no post is available
	if len(id) > 0 {
		home, err := os.Getwd()
		check(err)

		if post == nil {
			mySite := loadSite(home, "test")
			postLoc := fmt.Sprintf("%s/%s/post/%s.md", mySite.location, mySite.ContentDir(), id)
			log.Printf("Looking for %s\n", postLoc)

			post, err = loadPost(postLoc)
			if err != nil {
				// TODO: allow us to do other things
				http.Error(w, "File not found!", 404)
				return
			}
		}

		renderAnything(w, "editPage", post)

	} else {
		http.Error(w, "File not found :(", 404)
	}

}

// NewPost - Create a new post
func NewPost(w http.ResponseWriter, req *http.Request) {
	// TODO: Require login

	log.Println("Got a hit on a new post!")

	status := new(siteStatus)
	status.Success = false
	status.Build = false
	status.Message = ""

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)
		log.Println("Got POST")

		slug := req.FormValue("title")

		if len(slug) > 0 {
			status.Build = true
			path, err := mySite.newPost(slug)
			check(err)

			post, err := loadPost(path)
			if err != nil {
				status.Message = "Could not edit post: " + err.Error()
				goto render
			}

			post.slug = slug
			post.draft = true
			err = post.SavePost()
			if err != nil {
				status.Message = "Could not save post: " + err.Error()
				goto render
			}

			if err != nil {
				status.Message = err.Error()
				goto render
			} else {
				editLoc := fmt.Sprintf("/edit/%s", slug)
				log.Printf("redirecting to %s\n", editLoc)
				http.Redirect(w, req, editLoc, http.StatusTemporaryRedirect)
			}
		} else {
			status.Message = "you need to specify a name!"
		}
	}

	// This is a failure point. Everything below this has to be safe.
render:
	renderAnything(w, "newPostPage", status)

}

// RemovePost - Remove a Post
func RemovePost(w http.ResponseWriter, req *http.Request) {
	// TODO: Require login

	status := new(siteStatus)
	log.Println("Got a hit on a remove!")

	id := req.URL.Path[len("/delete/"):]
	if len(id) == 0 {
		http.Error(w, "File not found :'(", 404)
	}
	status.Message = id

	fileLoc := path.Join(mySite.location, "content/post", id+".md")
	if _, err := os.Stat(fileLoc); os.IsNotExist(err) {
		http.Error(w, "File not found :'(", 404)
		return
	}

	pageConfirmQuery := req.URL.Query()
	confirmation := pageConfirmQuery.Get("confirm")

	if confirmation == "yes" {
		log.Printf("Deleting %s\n", id)
		err := os.Remove(fileLoc)
		if err != nil {
			http.Error(w, "Couldn't delete file: "+err.Error(), 500)
		}
		http.Redirect(w, req, "/posts/", http.StatusTemporaryRedirect)
	}

	renderAnything(w, "deletePage", status)
}

// EditSite - Edit a site's configuration
func EditSite(w http.ResponseWriter, req *http.Request) {
	// TODO: Require login
	log.Println("Got a hit on a siteedit!")

	// TODO: Support multiple sites
	wrapper := new(WebWrapper)
	wrapper.Site = mySite
	wrapper.Config = mySite.SiteInfo()
	wrapper.Success = false

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)

		// get values
		values := req.Form

		mySite.builddrafts = false
		mySite.canonifyurls = false

		for i, v := range values {
			fmt.Printf("i: %s; v: %s\n", i, v)

			value := v[0]

			switch i {
			case "title":
				mySite.title = value
			case "subtitle":
				mySite.subtitle = value
			case "baseurl":
				mySite.baseurl = value
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
			default:
				// If it isn't one of the above, it must be something else that isn't as important.
				// TODO: Handle params prettier
				if strings.Contains(i, "params.") {
					log.Printf("Setting params value at %s to %s\n", i[len("params."):], value)
					mySite.params[i[len("params."):]] = value
				} else {
					mySite.allSettings[i] = value
					log.Printf("Setting hashmap value at %s to %s\n", i, value)
				}
			}
		}

		// save site
		err := mySite.SaveConfig()
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		wrapper.Success = true
	}

	renderAnything(w, "siteConfig", wrapper)
}

// AdvancedConfig - Edit a site's configuration (for power users)
func AdvancedConfig(w http.ResponseWriter, req *http.Request) {
	// TODO: Require login
	log.Println("Got a hit on a siteedit!")

	// TODO: Support multiple sites
	wrapper := new(WebWrapper)
	wrapper.Site = mySite
	wrapper.Config = mySite.SiteInfo()
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
