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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
 * @param databasePath (string): The path to a database file.
 *		(doesn't even need to exist)
 * @return userManager (userManager): An instance of userManager.
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
 * Hashes any given input using bcrypt.
 *
 * @param this (byte slice): Input.
 * @return hash (byte slice): Output.
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
 * @param err (error): The error object.
 **/
func (um *userManager) Check(err error) {
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			if um.debug {
				fmt.Println("[userManager][Debug]: Path error occured, creating database now.")
			}
			os.Create(um.databasePath)
		} else {
			if um.debug {
				fmt.Println("[userManager][Debug] Error: " + err.Error() + ". Panicking now!")
			}
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
 * @param user (string): The username.
 * @param pass (string): The password.
 * @return (bool): false if the username already exists. true if the user
 * registered successfully.
 **/
func (um *userManager) Register(user string, pass string) bool {
	if _, exists := um.users[user]; exists {
		return false
	}

	um.users[user] = um.Hash([]byte(pass))

	f, err := os.OpenFile(um.databasePath, os.O_APPEND|os.O_WRONLY, 0666)
	defer f.Close()
	um.Check(err)

	_, err = f.WriteString(user + ":" + string(um.users[user]) + "\n")
	um.Check(err)

	if um.debug {
		fmt.Println("[userManager][Debug] Registered user[" + user + "] with password[" + string(um.users[user]) + "].")
	}

	return true
}

/**
 * Changes the password of a user, given his old password matches the oldpass input variable.
 * Writes both into the database file and the memory.
 *
 * @param user (string): The username.
 * @param oldpass (string): The old password.
 * @param newpass (string): The new password.
 * @return (bool): false if the old password given doesn't match the actual old password, or
 * if the user doesn't exist. true if the password has changed.
 **/
func (um *userManager) ChangePass(user string, oldpass string, newpass string) bool {
	if um.CheckHash(user, []byte(oldpass)) {
		oldpass := string(um.users[user])
		um.users[user] = um.Hash([]byte(newpass))

		data, err := ioutil.ReadFile(um.databasePath)
		um.Check(err)

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, user) && strings.Contains(line, oldpass) {
				lines[i] = user + ":" + string(um.users[user])
				break
			}
		}

		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(um.databasePath, []byte(output), 0666)
		um.Check(err)

		if um.debug {
			fmt.Println("[userManager][Debug] Changed the password of user[" + user + "],\n\t" +
				"from[" + oldpass + "] to[" + string(um.users[user]) + "].")
		}

		return true
	}

	if um.debug {
		fmt.Println("[userManager][Debug] Failed to change the password of user[" + user + "].")
	}

	return false
}

/**
 * Changes the lifespan of a session.
 * Mainly exists to handle the typecasting between int and int64.
 *
 * @param seconds (int): The desired value in seconds.
 **/
func (sess *session) SetLifespan(seconds int) {
	sess.lifespan = int64(seconds)
}

/**
 * Pretty much self-explained. Checks if a session is logged.
 *
 * @return (bool): true if logged, false if not.
 **/
func (sess *session) IsLogged() bool {
	if sess.user != "" {
		return true
	}

	return false
}

/**
 * Checks for expired sessions.
 **/
func (um *userManager) CheckSessions() {
	for hash, sess := range um.sessions {
		if sess.timestamp+sess.lifespan < time.Now().Unix() {
			if um.debug {
				fmt.Println("[userManager][Debug] Session[" + hash + "] has expired. \n")
			}

			delete(um.sessions, hash)
		}
	}
}

/**
 * Uses SHA256 to hash a session token (the sum of identifiers).
 *
 * @param token (string): The input.
 * @return (string): The hash as string.
 **/
func (um *userManager) HashSessionToken(token string) string {
	hash := sha256.New()
	hash.Write([]byte(token))
	return hex.EncodeToString(hash.Sum(nil))
}

// GetSessionFromRequest - Get the user's session from their Request using
// the user's session cookie. If there is no session, returns an error and
// nil session. If the user's session actually exists, then return an actual
// session with a nil error.
func (um *userManager) GetSessionFromRequest(w http.ResponseWriter, req *http.Request) (*session, error) {
	loginCookie, err := req.Cookie("sessionID")
	if err != nil {
		log.Println("No session found!")
		log.Printf("error: %s\n", err.Error())
		return nil, err
	}

	userSessionTest := new(session)
	userSessionTest.ip = req.RemoteAddr
	userSessionTest.ua = req.UserAgent()

	sessionIDStr := loginCookie.Value
	fmt.Printf("Session id: %s\n", sessionIDStr)

	if realSession, exists := um.sessions[sessionIDStr]; exists {
		log.Println("Looking for matching session")
		if userSessionTest.ip == realSession.ip &&
			userSessionTest.ua == realSession.ua {
			return realSession, nil
		}

	}
	log.Println("No matching session found.")

	// Get rid of invalid cookie
	expiredCookie := new(http.Cookie)
	expiredCookie.Expires = time.Unix(0, 0)
	expiredCookie.Name = "sessionID"
	expiredCookie.Value = ""
	http.SetCookie(w, expiredCookie)

	log.Println("Set cookie!")

	return nil, fmt.Errorf("The user's session does not exist!\n")
}

// LoginCookie - Login a user. If the user is successfully logged in, give them
// a cookie to say logged in with. Return true if the login is successful.
func (um *userManager) LoginCookie(user string, pass string, w http.ResponseWriter, req *http.Request) (*http.Cookie, bool) {
	log.Println("Checking login.")

	if um.CheckHash(user, []byte(pass)) {
		log.Println("Login matched.")
		if um.debug {
			fmt.Println("[userManager][Debug] User[" + user + "] has logged in.")
		}

		newSession := new(session)
		newSession.ip = req.RemoteAddr
		newSession.ua = req.UserAgent()
		newSession.user = user
		newSession.SetLifespan((int)(defaultLifespan))
		newSession.timestamp = time.Now().Unix()

		randomID := make([]byte, 32)
		_, err := rand.Read(randomID)
		if err != nil {
			log.Fatal("Could not generate new randomID")
		}
		randomIDStr := base64.StdEncoding.EncodeToString(randomID)
		fmt.Printf("ID String: %s\n", randomIDStr)
		um.sessions[randomIDStr] = newSession

		// WTF IS THIS AND WHY DOESN'T IT WORK?
		loginCookie := new(http.Cookie)
		loginCookie.Expires = time.Now().Add(time.Duration(defaultLifespan))
		loginCookie.Name = "sessionID"
		loginCookie.Value = randomIDStr
		loginCookie.Secure = true
		loginCookie.HttpOnly = true

		http.SetCookie(w, loginCookie)

		return loginCookie, true
	}

	log.Println("Login did not match.")
	if um.debug {
		fmt.Println("[userManager][Debug] Attempted user[" + user + "] failed to log in.")
	}

	return nil, false
}
