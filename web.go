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
	"encoding/base64"
	"fmt"
	"github.com/niemal/uman"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	fiveMegabytes = (int64)(5 * 1024 * 1024)
)

// WebWrapper is a Struct for passing values to the web templates that Shim uses.
// This has a set of globally used values, as well as information about what
// happened for a page.
//
// There are fields which the developer may always consider to be non-nil, and
// there are values which must be assigned that are by default nil.
type WebWrapper struct {
	// These values are always populated and non-nil no matter what whenever
	// calling `NewWrapper`
	AllSites []*Site
	Site     *Site
	URL      string
	Action   string
	Message  string

	// Even though these things are opposite, they imply different things
	Success bool
	Failed  bool

	// These fields are route and page specific.
	Session  *uman.Session // Not populated by default
	Choices  []string      // Not populated by default
	Post     *Post         // Not populated by default
	Text     *bytes.Buffer // Not populated by default
	Anything interface{}   // Not populated by default
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
func NewWrapper(w http.ResponseWriter, req *http.Request) *WebWrapper {
	wr := new(WebWrapper)

	wr.URL = req.URL.String()

	session := um.GetHTTPSession(w, req)
	if session != nil && session.IsLogged() {
		site := findUserSite(session.User)
		wr.Site = site
	} else {
		wr.Site = allSites[0]
	}

	return wr
}

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
	status := NewWrapper(w, req)
	status.AllSites = allSites

	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			status.FailedMessage("Couldn't parse form!")
		} else {

			req.ParseForm()

			doBuild := strings.Trim(req.FormValue("doBuild"), " ")
			doPreview := strings.Trim(req.FormValue("doPreview"), " ")
			doReload := strings.Trim(req.FormValue("doReload"), " ")
			switchSite := strings.Trim(req.FormValue("switchSite"), " ")
			if len(doBuild) >= 1 {
				status.Action = "build"
			} else if len(doReload) >= 1 {
				status.Action = "reload"
			} else if len(doPreview) >= 1 {
				status.Action = "preview"
			} else if len(switchSite) >= 1 {
				status.Action = "switch"
			}
		}
	}

	if status.Action == "build" || status.Action == "preview" {
		var err error
		if status.Action == "build" {
			err = status.Site.BuildPublic()
		} else {
			err = status.Site.BuildPreview()
		}
		if err != nil {
			status.FailedMessage("Build failed. Reason: " + err.Error())
		} else {
			status.SuccessMessage("Build completed!")
		}
	} else if status.Action == "reload" {
		status.Site.Reload()
		status.SuccessMessage("Site reloaded.")
	} else if status.Action == "switch" {
		newSite := strings.TrimSpace(req.FormValue("newSite"))
		if newSite == status.Site.ShortName() {
			status.FailedMessage("You're already using that site!")
		} else {
			session := um.GetHTTPSession(w, req)
			for _, s := range allSites {
				if s.ShortName() == newSite {
					uName := session.User
					delete(userSites, uName)
					userSites[uName] = newSite
					status.Site.Reload()
					status.Site = s
					// Go ahead and build a preview in the background
					go status.Site.BuildPreview()
					status.SuccessMessage("Switched to site.")
					break
				}
			}
		}
	}

	renderAnything(w, "adminPage", status)
}

// Users - User Management Page
func Users(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(w, req)
	wrapper.Anything = um
	session := um.GetHTTPSession(w, req)
	wrapper.Session = session

	if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			wrapper.FailedMessage("Unable to parse your submission: " + err.Error())
		}

		accountAction := req.Form.Get("accountAction")

		fmt.Printf("accountAction: %s\n", accountAction)

		if accountAction == "changepass" {
			wrapper.Action = "changepass"
			oldPass := req.Form.Get("oldPass")
			newPass := req.Form.Get("newPass")
			newPassConfirm := req.Form.Get("newPassConfirm")

			if newPass == newPassConfirm {
				newPassLen := len(newPass)
				if newPassLen < 6 {
					wrapper.FailedMessage("Sorry, but your password needs to be at " +
						"least 6 characters long.")
				} else if newPassLen > 128 {
					wrapper.FailedMessage("128 bytes of password out to be enough for " +
						"everybody... Please use a password that's ≤ 128 characters.")
				} else if oldPass == newPass {
					wrapper.FailedMessage("Your old and new passwords cannot match!")
				} else {
					changed := um.ChangePass(session.User, oldPass, newPass)
					if changed {
						wrapper.SuccessMessage("Password successfully changed!")
					} else {
						wrapper.FailedMessage("Your current password was " +
							"incorrect. Please try again.")
					}
				}
			} else {
				wrapper.FailedMessage("Sorry, but your new password and its " +
					"confirmation don't match! Please try again.")
			}
		}
	}

	renderAnything(w, "userPage", wrapper)
}

// ViewTaxonomies Taxonomy management page
func ViewTaxonomies(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(w, req)

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
			session.SetLifespan(3600 * 24 * 7) // 1 Week (in seconds)
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
	wrapper := NewWrapper(w, req)
	wrapper.Site.GetAllPosts()

	renderAnything(w, "postsPage", wrapper)
}

// ViewFiles - View all files uploaded to this site
func ViewFiles(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(w, req)

	if req.Method == "POST" {
		req.ParseMultipartForm(fiveMegabytes)

		removeFile := req.Form.Get("removeFile")

		form := req.MultipartForm

		if form != nil {
			for name, value := range form.File {
				fmt.Printf("form field: %s\n", name)
				for _, v := range value {
					fmt.Printf("filename: %s\n", v.Filename)
					file, err := v.Open()
					defer file.Close()
					err = wrapper.Site.AddStaticFile(v.Filename, file)
					if err != nil {
						wrapper.FailedMessage("Failed to upload file: " + err.Error())
						break
					}
					wrapper.SuccessMessage("File uploaded.")
				}
			}
		}

		if len(removeFile) > 0 {
			err := wrapper.Site.RemoveStaticFile(removeFile)
			if err != nil {
				wrapper.FailedMessage("Unable to remove file. Error: " + err.Error())
			} else {
				wrapper.SuccessMessage("Removed file successfully.")
			}
		}
	} else {
		q := req.URL.Query()
		embedFile := q.Get("embed")
		fmt.Printf("embed file: %s\n", embedFile)

		if len(embedFile) > 0 {
			wrapper.Action = "embed"
			embed := wrapper.Site.GetEmbedCode(embedFile)
			wrapper.Message = embed
		}
	}

	renderAnything(w, "filesPage", wrapper)
}

// EditPost - Edit a Post
func EditPost(w http.ResponseWriter, req *http.Request) {
	wrapper := NewWrapper(w, req)

	// postID is the base64 post ID of a post.
	postID := req.URL.Path[len("/edit/"):]

	if len(postID) == 0 {
		http.Redirect(w, req, "/posts/", http.StatusTemporaryRedirect)
		return
	}

	postPathBytes, err := base64.StdEncoding.DecodeString(postID)
	if err != nil {
		http.Error(w, "Sorry, but the post ID you are trying to edit is invalid.",
			http.StatusNotFound)
		return
	}

	postPath := string(postPathBytes)

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
			case "aliases":
				individualValues := strings.Split(value, ",")
				stripChars(&individualValues, " ")
				removeDuplicates(&individualValues)
				if len(individualValues) == 0 && len(individualValues[0]) < 1 {
					// ignore taxonomy list with spaces only
				} else {
					post.aliases = individualValues
				}
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
	wrapper := NewWrapper(w, req)

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
				editLoc := fmt.Sprintf("/edit/%s", post.PostID())
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
	wrapper := NewWrapper(w, req)

	postID := req.URL.Path[len("/delete/"):]

	if len(postID) == 0 {
		http.Redirect(w, req, "/posts/", http.StatusTemporaryRedirect)
		return
	}

	postPathBytes, err := base64.StdEncoding.DecodeString(postID)
	if err != nil {
		http.Error(w, "Sorry, but the post ID you are trying to edit is invalid.",
			http.StatusNotFound)
		return
	}

	relPath := string(postPathBytes)

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
	wrapper := NewWrapper(w, req)

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
	wrapper := NewWrapper(w, req)

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
