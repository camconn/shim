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
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var mySite *Site
var shimAssets *assets
var um *userManager

// Location of assets on disk for Shim
type assets struct {
	root      string
	sites     string
	templates string
	static    string
	themes    string
}

// Pretty methods to check errors
func check(err error) {
	checkReason(err, "An error occurred: ")
}
func checkReason(err error, reason string) {
	if err != nil {
		log.Fatalf("%s\nDebug: %s\n", reason, err.Error())
	}
}

func assignAssets() {
	shimAssets = new(assets)

	root, err := os.Getwd()
	checkReason(err, "Couldn't find current working directory.")

	viper.SetDefault("sitesDir", "sites")
	viper.SetDefault("templatesDir", "templates")
	viper.SetDefault("staticDir", "static")
	viper.SetDefault("themeDir", "themes")

	shimAssets.root = root
	shimAssets.sites = viper.GetString("sitesDir")
	shimAssets.templates = viper.GetString("templatesDir")
	shimAssets.static = viper.GetString("staticDir")
	shimAssets.themes = viper.GetString("themeDir")
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

	// Make sure that *some* site exists
	setupTestSite()

	// Setup assets and appropriate folders
	assignAssets()

	um = umInit(filepath.Join(shimAssets.root, "users.db"))
	um.Register("root", "hunter2") // Super secure initial password

	fmt.Printf("Root directory is: %s\n", shimAssets.root)

	// For now, have a fixed site to load
	mySite = loadSite(shimAssets.root, "test")
	fmt.Printf("site: %s\n", mySite.String())

	// Things for running webapp
	mux := http.NewServeMux()

	loginRequirer := newLoginHandler(um).authHandler
	withAuth := alice.New(timeoutHandler, loggingHandler, loginRequirer)

	mux.Handle("/", withAuth.ThenFunc(Home))
	mux.Handle("/config/", withAuth.ThenFunc(EditSite))
	mux.Handle("/config/advanced/", withAuth.ThenFunc(AdvancedConfig))
	mux.Handle("/posts/", withAuth.ThenFunc(ViewPosts))
	mux.Handle("/edit/", withAuth.ThenFunc(EditPost))
	mux.Handle("/delete/", withAuth.ThenFunc(RemovePost))
	mux.Handle("/new/", withAuth.ThenFunc(NewPost))
	mux.Handle("/admin/", withAuth.ThenFunc(Admin))

	noAuth := alice.New(timeoutHandler, loggingHandler)
	staticFilesRoot := filepath.Join(shimAssets.root, shimAssets.static)
	staticFileHandler := http.FileServer(http.Dir(staticFilesRoot))

	mux.Handle("/login/", noAuth.ThenFunc(Login))
	mux.Handle("/static/", http.StripPrefix("/static/", noAuth.Then(staticFileHandler)))

	// Get port from environment variable for compatibility with gin for easy reloads
	portEnv := os.Getenv("PORT")
	if len(portEnv) != 0 {
		fmt.Printf("portenv: %s\n", portEnv)
	} else {
		portEnv = "8080"
	}
	fmt.Printf("portenv: %s\n", portEnv)

	mServ := http.Server{}
	addr := fmt.Sprintf(":%s", portEnv)
	mServ.Addr = fmt.Sprintf(addr)
	mServ.Handler = mux
	err = mServ.ListenAndServe()
	if err != nil {
		log.Fatalf("Error serving: %s\n", err.Error())
	}
}
