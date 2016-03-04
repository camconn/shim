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
	"github.com/justinas/alice"
	"github.com/niemal/uman"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var allSites []*Site
var mySite *Site
var shimAssets *assets
var um *uman.UserManager

// Pretty methods to check errors
func check(err error) {
	checkReason(err, "An error occurred: ")
}

func checkReason(err error, reason string) {
	if err != nil {
		log.Fatalf("%s\nDebug: %s\n", reason, err.Error())
	}
}

func main() {
	// Ensure that config exists before we try and load it.
	setupConfig()

	// Loading configuration
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config. Does 'config.toml' not exist?\nError: %s\n", err.Error())
	}

	// Setup assets and appropriate folders
	assignAssets()

	um = uman.New(filepath.Join(shimAssets.root, "users.db"))
	um.CheckDelay = 60
	um.Register("root", "hunter2") // Super secure initial password

	fmt.Printf("Root directory is: %s\n", shimAssets.root)

	// Lot sites and whatnot
	siteNames, err := findSites()
	checkReason(err, "Was not able to find primary site. Please check your `config.toml` file.")
	setupSites(siteNames)
	allSites = loadAllSites(siteNames)
	mySite = allSites[0]
	fmt.Printf("all sites: %##v\n", allSites)
	fmt.Printf("primary site: %s\n", mySite.String())

	// Below this line are things exclusively for running the webapp
	mux := http.NewServeMux()

	loginRequirer := newLoginHandler(um).authHandler
	withAuth := alice.New(loggingHandler, loginRequirer)

	mux.Handle("/", withAuth.ThenFunc(Home))
	mux.Handle("/config/", withAuth.ThenFunc(EditSite))
	mux.Handle("/config/advanced/", withAuth.ThenFunc(AdvancedConfig))
	mux.Handle("/posts/", withAuth.ThenFunc(ViewPosts))
	mux.Handle("/edit/", withAuth.ThenFunc(EditPost))
	mux.Handle("/delete/", withAuth.ThenFunc(RemovePost))
	mux.Handle("/new/", withAuth.ThenFunc(NewPost))
	mux.Handle("/admin/", withAuth.ThenFunc(Admin))
	mux.Handle("/taxonomy/", withAuth.ThenFunc(ViewTaxonomies))

	withPreview := alice.New(loggingHandler, loginRequirer)
	previewSiteRoot := filepath.Join(mySite.Location(), "preview")
	previewSiteHandler := http.StripPrefix("/preview/", http.FileServer(http.Dir(previewSiteRoot)))
	mux.Handle("/preview/", withPreview.Then(previewSiteHandler))

	noAuth := alice.New(loggingHandler)
	staticFilesRoot := filepath.Join(shimAssets.root, shimAssets.static)
	staticFileHandler := http.FileServer(http.Dir(staticFilesRoot))

	mux.Handle("/login/", noAuth.ThenFunc(Login))
	mux.Handle("/static/", http.StripPrefix("/static/", noAuth.Then(staticFileHandler)))

	// Get port from environment variable for compatibility with gin for easy reloads
	portEnv := os.Getenv("PORT")
	if len(portEnv) == 0 {
		portEnv = "8080"
	}
	fmt.Printf("portenv: %s\n", portEnv)

	fmt.Printf("baseurl: %s\n", mySite.BaseURL())
	mServ := http.Server{}
	addr := fmt.Sprintf(":%s", portEnv)
	mServ.Addr = fmt.Sprintf(addr)
	mServ.Handler = mux
	err = mServ.ListenAndServe()
	if err != nil {
		log.Fatalf("Error serving: %s\n", err.Error())
	}
}
