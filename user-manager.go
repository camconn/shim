// SHIM - A web front end for the Hugo site generator
// Copyright (C) 2016        Cameron Conn and friends

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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type session struct {
	user      string
	ip        string
	ua        string
	timestamp int64
	lifespan  int64
}

type userManager struct {
	users        map[string][]byte
	databasePath string
	sessions     map[string]*session
	debug        bool
}

const defaultLifespan int64 = 3600 * 12

/**
 * Constructor of userManager.
 *
 **/
func umInit(databasePath string) *userManager {
	userManager := new(userManager)

	userManager.databasePath = databasePath
	userManager.sessions = make(map[string]*session)
	userManager.debug = false
	userManager.Reload()

	return userManager
}

/**
 * Handles checking for debug mode and if so prints text.
 *
 **/
func (um *userManager) Debug(message string) {
	if um.debug {
		fmt.Println("[userManager]: " + message)
	}
}

/**
 * Hashes any given input using bcrypt.
 *
 **/
func (um *userManager) Hash(this []byte) []byte {
	// cost: minimum is 4, max is 31, default is 10
	// (https://godoc.org/golang.org/x/crypto/bcrypt)
	cost := 10

	hash, err := bcrypt.GenerateFromPassword(this, cost)
	um.Check(err)

	return hash
}

// CheckHash - Checks a hash against its possible plaintext. This exists because of
// bcrypt's mechanism, we shouldn't just um.Hash() and check it ourselves.
// If the password `test` is correct, then return true. Otherwise, return false.
func (um *userManager) CheckHash(user string, test []byte) bool {
	realHash, exists := um.users[user]
	if exists {
		if bcrypt.CompareHashAndPassword(realHash, test) != nil {
			return false
		}
		return true
	}
	return false
}

/**
 * Internal userManager error handling. Specifically, if a path error occurs
 * that means we just need to create the database. Furthermore in debug mode
 * a stdout message pops before panic()'ing, due to a panic possibility occuring
 * out of the blue.
 *
 **/
func (um *userManager) Check(err error) {
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			um.Debug("Path error occured, creating database now.")
			os.Create(um.databasePath)
		} else {
			um.Debug(err.Error() + "\nPanicking.")
			panic(err)
		}
	}
}

/**
 * Reloads the database into memory, filling the appropriate variables.
 *
 **/
func (um *userManager) Reload() {
	um.users = make(map[string][]byte) // reset

	data, err := ioutil.ReadFile(um.databasePath)
	um.Check(err)

	accounts := strings.Split(string(data), "\n")
	for _, acc := range accounts {
		creds := strings.Split(acc, ":")

		if len(creds) != 2 {
			// at times for unknown reasons the length of creds is 1
			// which causes an unchecked panic to pop. it shouldn't mind us.
			break
		}

		um.users[creds[0]] = []byte(creds[1])
	}
}

/**
 * Registers a new user, writing both into the database file and the memory.
 *
 **/
func (um *userManager) Register(user string, pass string) bool {
	if _, exists := um.users[user]; exists {
		return false
	}

	um.users[user] = um.Hash([]byte(pass))
	pass = string(um.users[user])

	f, err := os.OpenFile(um.databasePath, os.O_APPEND|os.O_WRONLY, 0666)
	defer f.Close()
	um.Check(err)

	_, err = f.WriteString(user + ":" + pass + "\n")
	um.Check(err)

	um.Debug("Registered user[" + user + "] with password[" + pass + "].")
	return true
}

/**
 * Changes the password of a user, given his old password matches the oldpass input variable.
 * Writes both into the database file and the memory.
 *
 * Returns false if the old password given doesn't match the actual old password, or user
 * doesn't exist.
 *
 **/
func (um *userManager) ChangePass(user string, oldpass string, newpass string) bool {
	if um.CheckHash(user, []byte(oldpass)) {
		oldpass := string(um.users[user])
		um.users[user] = um.Hash([]byte(newpass))
		newpass = string(um.users[user])

		data, err := ioutil.ReadFile(um.databasePath)
		um.Check(err)

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, user) && strings.Contains(line, oldpass) {
				lines[i] = user + ":" + newpass
				break
			}
		}

		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(um.databasePath, []byte(output), 0666)
		um.Check(err)

		um.Debug("Changed the password of user[" + user + "],\n\t" +
			"from[" + oldpass + "] to[" + newpass + "].")
		return true
	}

	um.Debug("Failed to change the password of user[" + user + "].")
	return false
}

/**
 * Changes the lifespan of a session.
 * Mainly exists to handle the typecasting between int and int64.
 *
 **/
func (sess *session) SetLifespan(seconds int) {
	sess.lifespan = int64(seconds)
}

/**
 * Checks if a session is logged.
 *
 **/
func (sess *session) IsLogged() bool {
	if sess.user != "" {
		return true
	}

	return false
}

/**
 * Checks for expired sessions.
 *
 **/
func (um *userManager) CheckSessions() {
	for hash, sess := range um.sessions {
		if sess.timestamp+sess.lifespan < time.Now().Unix() {
			um.Debug("Session[" + hash + "] has expired.")
			delete(um.sessions, hash)
		}
	}
}

// GetSessionFromRequest - Get the user's session from their Request using
// the user's session cookie. If there is no session, returns an error and
// nil session. If the user's session actually exists, then return an actual
// session with a nil error.
func (um *userManager) GetSessionFromRequest(w http.ResponseWriter, req *http.Request) (*session, error) {
	loginCookie, err := req.Cookie("sessionID")
	if err != nil {
		um.Debug("No session cookie found.")
		return nil, err
	}

	userSessionTest := new(session)
	userSessionTest.ip = req.RemoteAddr
	userSessionTest.ua = req.UserAgent()

	sessionIDStr := loginCookie.Value
	um.Debug("Test session id: " + sessionIDStr)

	// Sessions are based on the user's IP and User Agent. If either of these
	// differs, then the session is invalid. Thus, let's make it invalid.
	if realSession, exists := um.sessions[sessionIDStr]; exists {
		if userSessionTest.ip == realSession.ip &&
			userSessionTest.ua == realSession.ua {
			um.Debug("Found valid session.")
			return realSession, nil
		}
	}

	um.Debug("No matching session found. Removing invalid cookie.")

	expiredCookie := new(http.Cookie)
	expiredCookie.MaxAge = 0
	expiredCookie.Expires = time.Unix(0, 0)
	expiredCookie.Name = "sessionID"
	expiredCookie.Value = ""
	expiredCookie.Path = "/"
	http.SetCookie(w, expiredCookie)

	return nil, fmt.Errorf("The user's session does not exist!\n")
}

// LoginCookie - Login a user. If the user is successfully logged in, return a cookie
// and a true boolean value. If the user isn't logged in, then return nil and a false
// boolean value.
func (um *userManager) LoginCookie(user string, pass string, w http.ResponseWriter, req *http.Request) (*http.Cookie, bool) {
	log.Println("Checking login.")

	if um.CheckHash(user, []byte(pass)) {
		um.Debug("[userManager][Debug] User[" + user + "] has logged in.")

		newSession := new(session)
		newSession.ip = req.RemoteAddr
		newSession.ua = req.UserAgent()
		newSession.user = user
		newSession.SetLifespan((int)(defaultLifespan))
		newSession.timestamp = time.Now().Unix()

		randomID := make([]byte, 32)
		_, err := rand.Read(randomID) // yes, crypto/rand is secure for this
		if err != nil {
			log.Fatal("Could not generate new randomID")
		}
		// We encode the id using base64 so that it only contains characters which
		// are valid in cookies.
		randomIDStr := base64.StdEncoding.EncodeToString(randomID)
		um.Debug("ID String: " + randomIDStr)
		um.sessions[randomIDStr] = newSession

		loginCookie := new(http.Cookie)
		expires := time.Now().Add(time.Duration(defaultLifespan))
		loginCookie.Expires = expires
		loginCookie.Name = "sessionID"
		loginCookie.Value = randomIDStr
		loginCookie.Path = "/"
		loginCookie.HttpOnly = true
		loginCookie.MaxAge = (int)(defaultLifespan)
		// loginCookie.Secure = true  // TODO: Is this needed?

		return loginCookie, true
	}

	um.Debug("[userManager][Debug] Attempted user[" + user + "] failed to log in.")

	return nil, false
}
