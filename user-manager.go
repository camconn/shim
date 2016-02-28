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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type session struct {
	user      string
	timestamp int64
	lifespan  int64
}

type userManager struct {
	users        map[string][]byte
	databasePath string
	sessions     map[string]*session
	debug        bool
}

const defaultLifespan int64 = 3600

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

/**
 * Checks a hash against its possible plaintext. This exists because of
 * bcrypt's mechanism, we shouldn't just um.Hash() and check it ourselves.
 *
 **/
func (um *userManager) CheckHash(hash []byte, original []byte) bool {
	if bcrypt.CompareHashAndPassword(hash, original) != nil {
		return false
	}

	return true
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
	if um.CheckHash(um.users[user], []byte(oldpass)) {
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

		um.Debug("Changed the password of user[" + user + "],\n\t" +
			"from[" + oldpass + "] to[" + string(um.users[user]) + "].")
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

/**
 * Uses SHA256 to hash a session token (the sum of identifiers).
 *
 **/
func (um *userManager) HashSessionToken(token string) string {
	hash := sha256.New()
	hash.Write([]byte(token))
	return hex.EncodeToString(hash.Sum(nil))
}

/**
 * Generates a new session or returns the existing one.
 * It uses 2 identifiers, ua and id, one could translate those variables into
 * user agent and IP address.
 *
 **/
func (um *userManager) GetSession(ua string, id string) *session {
	hash := um.HashSessionToken(ua + id)

	if _, exists := um.sessions[hash]; exists {
		return um.sessions[hash]
	}

	sess := new(session)
	sess.lifespan = defaultLifespan
	sess.timestamp = time.Now().Unix()

	um.sessions[hash] = sess

	return sess
}

/**
 * Attempts to log in a user.
 *
 **/
func (um *userManager) Login(user string, pass string, sess *session) bool {
	if um.CheckHash(um.users[user], []byte(pass)) {
		sess.user = user

		um.Debug("User[" + user + "] has logged in.")
		return true
	}

	um.Debug("Attempted user[" + user + "] failed to log in.")
	return false
}
