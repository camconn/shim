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
	"io/ioutil"
	"golang.org/x/crypto/bcrypt"
	"crypto/sha256"
	"encoding/hex"
	"time"
	"os"
	"strings"
	"fmt"
)

type session struct {
	user string
	timestamp int64
	lifespan int64
}

type userManager struct {
	users map[string][]byte
	databasePath string
	sessions map[string]*session
	debug bool
}

const DEFAULT_LIFESPAN int64 = 3600


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

/**
 * Checks a hash against its possible plaintext. This exists because of
 * bcrypt's mechanism, we shouldn't just um.Hash() and check it ourselves.
 *
 * @param hash (byte slice): The existing hash.
 * @param origin (byte silce): The possible plaintext.
 * @return (bool): The result.
 **/
func (um *userManager) CheckHash(hash []byte, original []byte) bool {
	if bcrypt.CompareHashAndPassword(hash, original) != nil {
		return false
	} else {
		return true
	}
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

	f, err := os.OpenFile(um.databasePath, os.O_APPEND | os.O_WRONLY, 0666)
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
		if sess.timestamp + sess.lifespan < time.Now().Unix() {
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

/**
 * Generates a new session or returns the existing one.
 * It uses 2 identifiers, ua and id, one could translate those variables into
 * user agent and IP address.
 *
 * @param ua (string): Identifier #1 (user agent).
 * @param id (string): Identifier #2 (IP address).
 * @return (session): The session object.
 **/
func (um *userManager) GetSession(ua string, id string) *session {
	hash := um.HashSessionToken(ua + id)

	if _, exists := um.sessions[hash]; exists {
		return um.sessions[hash]
	} else {
		sess := new(session)
		sess.lifespan = DEFAULT_LIFESPAN
		sess.timestamp = time.Now().Unix()

		um.sessions[hash] = sess

		return sess
	}
}

/**
 * Attempts to log in a user.
 *
 * @param user (string): Username.
 * @param pass (string): Password.
 * @param sess (session): The current session object.
 * @return (bool): false if wrong credentials combination, true if user has logged in.
 **/
func (um *userManager) Login(user string, pass string, sess *session) bool {
	if um.CheckHash(um.users[user], []byte(pass)) {
		sess.user = user

		if um.debug {
			fmt.Println("[userManager][Debug] User[" + user + "] has logged in.")
		}

		return true
	} else {
		if um.debug {
			fmt.Println("[userManager][Debug] Attempted user[" + user + "] failed to log in.")
		}

		return false
	}
}