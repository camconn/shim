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
	"strings"
	"time"
)

// WebWrapper - Struct for passing values to the web
type WebWrapper struct {
	Message string
	Action  string
	Site    *Site
	Post    *Post
	Text    *bytes.Buffer
	Choices []string // TODO: Remove this and use a ConfigOption instead
	URL     string
	// Even though these things are opposite, they imply different things
	Success bool
	Failed  bool
}

// SuccessMessage Modify WebWrapper to show a success message.
func (w *WebWrapper) SuccessMessage(message string) {
	w.Message = message
	w.Success = true
}

// FailedMessage Modify WebWrapper to show a failure message.
func (w *WebWrapper) FailedMessage(message string) {
	w.Message = message
	w.Failed = true
}

// NewWrapper Creates a new WebWrapper struct appropriate to the context of the
// user, taking into account the current site as well as the URL
func NewWrapper(req *http.Request) *WebWrapper {
	w := new(WebWrapper)
	// TODO: Switch sites based on cookies and whatnot
	w.Site = allSites[0]
	w.URL = req.URL.String()
	return w
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
	status := NewWrapper(req)

	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			status.FailedMessage("Couldn't parse form!")
		} else {

			req.ParseForm()

			doBuild := strings.Trim(req.FormValue("doBuild"), " ")
			doReload := strings.Trim(req.FormValue("doReload"), " ")
			if len(doBuild) >= 1 {
				status.Action = "build"
			} else if len(doReload) >= 1 {
				status.Action = "reload"
			}
		}
	}

	if status.Action == "build" {
		err := status.Site.BuildPublic()
		if err != nil {
			status.FailedMessage("Build failed. Reason: " + err.Error())
		} else {
			status.SuccessMessage("Build completed!")
		}
	} else if status.Action == "reload" {
		status.Site.Reload()
		status.SuccessMessage("Site reloaded.")
	}

	renderAnything(w, "adminPage", status)
}

// ViewTaxonomies Taxonomy management page
func ViewTaxonomies(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(req)

	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			wrapper.FailedMessage("Failed to update taxonomies")
			goto renderTaxonomy
		}

		name := req.Form.Get("kindName")
		newKind := req.Form.Get("newKind")
		delKind := req.Form.Get("deleteKind")

		if len(newKind) != 0 && len(delKind) != 0 {
			wrapper.FailedMessage("Umm. Your browser messed that request up. Sorry!")
			goto renderTaxonomy
		}

		if len(newKind) > 0 {
			wrapper.Action = "add"

			// Split into two names
			namePair := strings.SplitAfterN(name, ",", 2)
			stripChars(&namePair, ", ")
			removeDuplicates(&namePair)

			if len(namePair) != 2 {
				wrapper.FailedMessage("You need to enter a new name pair for this Taxonomy.")
				goto renderTaxonomy
			}

			plural := namePair[1]
			_, err := wrapper.Site.Taxonomies().GetTaxonomy(plural)
			if err == nil {
				wrapper.FailedMessage("Failed to create taxonomy because it already exists!")
				goto renderTaxonomy
			}

			fmt.Printf("Creating new taxonomy: (%s, %s)\n", namePair[0], namePair[1])
			wrapper.Site.Taxonomies().NewTaxonomy(namePair[0], namePair[1])
			// check the taxonomy was added
			_, err = wrapper.Site.Taxonomies().GetTaxonomy(namePair[1])
			if err != nil {
				wrapper.FailedMessage("Failed to create taxonomy.")
				goto renderTaxonomy
			}

			err = wrapper.Site.SaveConfig()
			if err != nil {
				wrapper.FailedMessage("Able to create taxonomy, but couldn't save it. Please try saving again.")
			} else {
				wrapper.SuccessMessage("Taxonomy created.")
			}
		} else if len(delKind) > 0 {
			wrapper.Action = "delete"
			if len(name) <= 0 {
				wrapper.FailedMessage("Unable to determine which taxonomy to delete.")
				goto renderTaxonomy
			}

			// check if the taxonomy actually exists
			_, err = wrapper.Site.Taxonomies().GetTaxonomy(name)
			if err != nil {
				wrapper.FailedMessage(fmt.Sprintf(
					"The taxonomy you want to delete (%s) doesn't exist!",
					name))
				goto renderTaxonomy
			}

			delete(wrapper.Site.Taxonomies(), name)

			err = wrapper.Site.SaveConfig()
			if err != nil {
				wrapper.FailedMessage("Was able to remove taxonomy, but couldn't update site configuration." +
					"Please try saving again.")
				goto renderTaxonomy
			}
			wrapper.SuccessMessage("Successfully removed Taxonomy.")
		} else {
			log.Println("WTF Happened here?")
			wrapper.FailedMessage("Unable to determine what taxonomy action to perform.")
		}
	}

renderTaxonomy:
	renderAnything(w, "taxonomyPage", wrapper)
}

// Login - The login page
func Login(w http.ResponseWriter, req *http.Request) {
	wrapper := new(WebWrapper)

	q := req.URL.Query()
	redirect := q.Get("redirect")
	if len(redirect) > 0 {

		warn := q.Get("warn")
		if len(warn) > 0 {
			wrapper.Action = "warn"
			q.Del("warn")
			wrapper.URL = "/login/?" + q.Encode()
			wrapper.Message = "Please login in."
		}
	} else {
		redirect = "/admin/" // By default, redirect to /admin/
	}

	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			wrapper.FailedMessage("Couldn't parse form!")
			renderAnything(w, "loginPage", &wrapper)
			return
		}

		username := req.FormValue("username")
		password := req.FormValue("password")

		session := um.GetHTTPSession(w, req)
		if success := um.Login(username, password, session); success {
			session.User = username
			session.SetLifespan(3600 * 24 * 24)
			session.SetHTTPCookie(w)

			log.Println("Redirecting to " + redirect)
			http.Redirect(w, req, redirect, http.StatusSeeOther)
			return
		}

		wrapper.FailedMessage("Incorrect username/password combination. Please try again.")
	}

	renderAnything(w, "loginPage", &wrapper)
}

// ViewPosts - View all posts
func ViewPosts(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(req)
	wrapper.Site.GetAllPosts()

	renderAnything(w, "postsPage", wrapper)
}

// EditPost - Edit a Post
func EditPost(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(req)
	postPath := req.URL.Path[len("/edit/"):]

	if len(postPath) == 0 {
		http.Redirect(w, req, "/posts/", http.StatusTemporaryRedirect)
		return
	}

	contentDirPath := filepath.Join(wrapper.Site.Location(), wrapper.Site.ContentDir())
	postLoc := fmt.Sprintf("%s.md", filepath.Join(contentDirPath, postPath))
	fmt.Printf("location: %s\n", postLoc)
	post, err := wrapper.Site.loadPost(postLoc, contentDirPath)
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
					wrapper.FailedMessage("Could not parse publishing time! " +
						"Please use a valid format, date, and time for time published.")
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
				if strings.Contains(i, "taxonomy.") {
					parts := strings.SplitAfterN(i, ".", 2)
					right := parts[1]
					if len(right) == 0 {
						// crap
						continue
					}

					individualValues := strings.Split(value, ",")
					stripChars(&individualValues, " ")
					removeDuplicates(&individualValues)
					post.taxonomies[right] = individualValues
				} else {
					log.Printf("edit post ignoring %s and %s.\n", i, value)
				}
			}
		}

		err = post.SavePost()
		if err != nil {
			log.Printf("Error while saving post: %s\n", err.Error())
			wrapper.FailedMessage("Could not save post to disk. Error: " + err.Error())
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
				wrapper.FailedMessage("Could not publish post: " + err.Error())
				goto renderEditedPost
			}
		} else {
			post.draft = true
		}

		wrapper.SuccessMessage("Post saved.")
	}

renderEditedPost:
	wrapper.Post = post

	renderAnything(w, "editPage", wrapper)
}

// NewPost - Create a new post
func NewPost(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(req)

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)
		newFilename := req.FormValue("title")

		if len(newFilename) > 0 {
			wrapper.Action = "build"

			contentDirPath := filepath.Join(wrapper.Site.Location(), wrapper.Site.ContentDir())
			testPostPath := filepath.Join(contentDirPath, "post", newFilename) + ".md"

			if _, err := os.Stat(testPostPath); !os.IsNotExist(err) {
				wrapper.FailedMessage("Post already exists!")
				goto render
			}

			path, err := wrapper.Site.newPost(newFilename)
			if err != nil {
				wrapper.FailedMessage("Could not edit post: " + err.Error())
				goto render
			}

			post, err := wrapper.Site.loadPost(path, contentDirPath)
			if err != nil {
				wrapper.FailedMessage("Could not edit post: " + err.Error())
				goto render
			}

			post.draft = true
			err = post.SavePost()
			if err != nil {
				wrapper.FailedMessage("Could not save post: " + err.Error())
				goto render
			}

			if err != nil {
				wrapper.FailedMessage("Could not save post: " + err.Error())
				goto render
			} else {
				editLoc := fmt.Sprintf("/edit/%s", post.RelPath())
				log.Printf("redirecting to %s\n", editLoc)
				http.Redirect(w, req, editLoc, http.StatusSeeOther)
				return
			}
		} else {
			wrapper.FailedMessage("you need to specify a name!")
		}
	}

render:
	renderAnything(w, "newPostPage", wrapper)

}

// RemovePost - Remove a Post
func RemovePost(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(req)

	relPath := req.URL.Path[len("/delete/"):]
	if len(relPath) == 0 {
		http.Error(w, "File not found :'(", 404)
		return
	}

	fileLoc := filepath.Join(wrapper.Site.Location(), wrapper.Site.ContentDir(), relPath+".md")
	if _, err := os.Stat(fileLoc); os.IsNotExist(err) {
		http.Error(w, "File not found :'(", 404)
		return
	}

	contentDirPath := filepath.Join(wrapper.Site.Location(), wrapper.Site.ContentDir())
	post, err := wrapper.Site.loadPost(fileLoc, contentDirPath)
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
			wrapper.FailedMessage("Couldn't delete file: " + err.Error())
		} else {
			http.Redirect(w, req, "/posts/", http.StatusSeeOther)
		}
	}

	renderAnything(w, "deletePage", wrapper)
}

// EditSite - Edit a site's basic configuration
func EditSite(w http.ResponseWriter, req *http.Request) {
	// TODO: Support multiple sites
	wrapper := NewWrapper(req)

	themesLoc := fmt.Sprintf("%s/%s", shimAssets.root, shimAssets.themes)
	allThemes, err := GetThemes(themesLoc)
	if err != nil {
		wrapper.FailedMessage(fmt.Sprintf("Failed to load themes: %s", err.Error()))
		goto renderBasicConfig
	}
	wrapper.Choices = allThemes

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)

		// get values
		values := req.Form

		wrapper.Site.builddrafts = false
		wrapper.Site.canonifyurls = false

		for i, v := range values {
			//fmt.Printf("i: %s; v: %s\n", i, v)

			value := v[0]

			switch i {
			case "title":
				wrapper.Site.title = value
			case "baseurl":
				wrapper.Site.baseurl = value
			case "theme":
				wrapper.Site.theme = value
			case "contentDir":
				wrapper.Site.contentDir = value
			case "layoutDir":
				wrapper.Site.layoutDir = value
			case "publishDir":
				wrapper.Site.publishDir = value
			case "builddrafts":
				wrapper.Site.builddrafts = true
			case "canonifyurls":
				wrapper.Site.canonifyurls = true

				// Now for site-wide params
			case "params.author":
				wrapper.Site.author = value
			case "params.subtitle":
				wrapper.Site.subtitle = value
			default:
				log.Printf("WTF IS %s and %s?\n", i, value)
			}
		}

		// save site
		err := wrapper.Site.SaveConfig()
		if err != nil {
			wrapper.FailedMessage(fmt.Sprintf("Failed to save site: %s", err.Error()))
			goto renderBasicConfig
		}

		err = ChangeTheme(wrapper.Site, wrapper.Site.Theme())
		if err != nil {
			wrapper.FailedMessage(fmt.Sprintf("Failed to change theme: %s", err.Error()))
			goto renderBasicConfig
		}

		wrapper.SuccessMessage("Site configuration has been updated.")
	}
renderBasicConfig:
	renderAnything(w, "siteConfig", wrapper)
}

// AdvancedConfig - Edit a site's configuration (for power users)
func AdvancedConfig(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(req)

	wrapper.Text = bytes.NewBuffer([]byte{})
	configLoc := filepath.Join(wrapper.Site.Location(), "config.toml")
	var file *os.File
	var err error

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)

		// get values
		values := req.Form
		configSrc := values.Get("configSrc")

		mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		file, err := os.OpenFile(configLoc, mode, 0666)
		if err != nil {
			wrapper.FailedMessage(fmt.Sprintf(
				"Settings could not be saved because an error occurred: %s", err.Error()))
			goto renderAdvancedConfig
		}
		defer file.Close()

		// save site
		file.WriteString(configSrc)
		if err != nil {
			wrapper.FailedMessage(fmt.Sprintf("Your settings may be corrupted -- "+
				"Settings could not be saved because an error occurred: %s", err.Error()))
			goto renderAdvancedConfig
		}

		wrapper.Site.Reload()
		wrapper.SuccessMessage("Successfully saved and reloaded configuration.")
	}

	wrapper.Text = bytes.NewBuffer([]byte{})
	file, err = os.Open(configLoc)
	defer file.Close()
	if err != nil {
		wrapper.FailedMessage(fmt.Sprintf(
			"Config could not be read because an error occurred: %s", err.Error()))
		goto renderAdvancedConfig
	}

	_, err = wrapper.Text.ReadFrom(file)
	if err != nil {
		wrapper.FailedMessage(fmt.Sprintf(
			"Config could not be read because an error occurred: %s", err.Error()))
		goto renderAdvancedConfig
	}

renderAdvancedConfig:
	renderAnything(w, "siteConfigAdvanced", wrapper)
}
