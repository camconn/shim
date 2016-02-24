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
	"container/list"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
)

var mySite *Site
var shimAssets *assets

// Location of assets on disk for Shim
type assets struct {
	root      string
	sites     string
	templates string
	static    string
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

	shimAssets.root = root
	shimAssets.sites = viper.GetString("sitesDir")
	shimAssets.templates = viper.GetString("templatesDir")
	shimAssets.static = viper.GetString("staticDir")
}

func main() {

	// Ensure that config exists before we try and load it.
	setupConfig()

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config. Does 'config.toml' not exist?\nError: %s\n", err.Error())
	}

	// Setup test sites, alright?
	setupTestSite()

	// Okay now load assets and sitess
	assignAssets()
	sites := viper.GetStringSlice("sites.all")

	enabled := list.New()
	for _, v := range sites {
		boolVal := "sites." + v + ".enabled"
		viper.SetDefault(boolVal, false)
		if viper.GetBool(boolVal) {
			fmt.Printf("%s is enabled\n", v)
			enabled.PushBack(v)
		}
	}

	// list of sites
	// sites := list.New()

	home, err := os.Getwd()
	check(err)
	fmt.Printf("Root directory is: %s\n", home)

	//mySiteElem := enabled.Front().Value
	//mySite, ok := mySiteElem.(Site)
	mySite = loadSite(home, "test")
	fmt.Printf("site: %s\n", mySite.String())

	checkReason(err, "Site build failed WTF")

	fmt.Println("Staring webapp")

	http.HandleFunc("/", Home)
	http.HandleFunc("/posts/", ViewPosts)
	http.HandleFunc("/edit/", EditPost)
	http.HandleFunc("/new/", NewPost)
	http.HandleFunc("/login/", Login)
	http.HandleFunc("/admin/", Admin)

	staticFilesRoot := fmt.Sprintf("%s/%s/", shimAssets.root, shimAssets.static)
	staticFileHandler := http.FileServer(http.Dir(staticFilesRoot))
	http.Handle("/static/", http.StripPrefix("/static/", staticFileHandler))

	// Get port from environment variable for compatibility with gin for easy reloads
	portEnv := os.Getenv("PORT")
	if len(portEnv) != 0 {
		fmt.Printf("portenv: %s\n", portEnv)
	} else {
		portEnv = "8080"
	}
	fmt.Printf("portenv: %s\n", portEnv)

	fmt.Printf("Now serving on http://127.0.0.1:%s\n", portEnv)
	listenAddress := fmt.Sprintf(":%s", portEnv)
	err = http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Fatal("ListenAndServer: ", err)
	}

}
