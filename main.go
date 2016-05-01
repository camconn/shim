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
var shimAssets *assets
var um *uman.UserManager

// Pretty methods to check errors
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
	checkReason(err, "Could no read config. Does 'config.toml' not exist?")

	// Setup assets and appropriate folders
	assignAssets()

	fmt.Printf("base path: %s\n", shimAssets.basepath)

	if firstRun() { // Setup initial username and password so admins can run shim.
		um = uman.New(filepath.Join(shimAssets.root, "users.db"))
		um.Register("root", "hunter2")
	} else {
		um = uman.New(filepath.Join(shimAssets.root, "users.db"))
	}
	um.CheckDelay = 60

	fmt.Printf("Root directory is: %s\n", shimAssets.root)

	// Lot sites and whatnot
	siteNames, err := findSites()
	checkReason(err, "Was not able to load sites. Please check your `config.toml` file.")
	setupSites(siteNames)
	allSites = loadAllSites(siteNames)

	// Below this line are things exclusively for running the webapp
	mux := http.NewServeMux()

	loginH := newLoginHandler(um)
	loginRequirer := loginH.authHandler
	withAuth := alice.New(loggingHandler, loginRequirer)

	mux.Handle("/", withAuth.ThenFunc(Home))
	mux.Handle("/config/", withAuth.ThenFunc(EditSite))
	mux.Handle("/config/advanced/", withAuth.ThenFunc(AdvancedConfig))
	mux.Handle("/posts/", withAuth.ThenFunc(ViewPosts))
	mux.Handle("/staticfiles/", withAuth.ThenFunc(ViewFiles))
	mux.Handle("/edit/", withAuth.ThenFunc(EditPost))
	mux.Handle("/delete/", withAuth.ThenFunc(RemovePost))
	mux.Handle("/new/", withAuth.ThenFunc(NewPost))
	mux.Handle("/admin/", withAuth.ThenFunc(Admin))
	mux.Handle("/user/", withAuth.ThenFunc(Users))
	mux.Handle("/taxonomy/", withAuth.ThenFunc(ViewTaxonomies))

	previewer := http.HandlerFunc(loginH.dynamicPreviewHandler)
	mux.Handle("/preview/", http.StripPrefix("/preview", withAuth.Then(previewer)))
	// workaround hack to serve preview static files correctly (e.g. images)
	mux.Handle("/files/", withAuth.Then(previewer))

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

	mServ := http.Server{}
	addr := fmt.Sprintf(":%s", portEnv)
	mServ.Addr = fmt.Sprintf(addr)
	mServ.Handler = http.StripPrefix(shimAssets.basepath, mux)
	err = mServ.ListenAndServe()
	checkReason(err, "Error serving webapp")
}
