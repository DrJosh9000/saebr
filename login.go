// Copyright 2020 Josh Deprez. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package saebr

import (
	"encoding/json"
	"log"
	"net/http"
)

const tokenVerifyURL = "https://oauth2.googleapis.com/tokeninfo?id_token="

type tokenVerification struct {
	Issuer        string `json:"iss"`
	AZP           string `json:"azp"`
	Audience      string `json:"aud"`
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified,string"`
	AtHash        string `json:"at_hash"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Locale        string `json:"locale"`
	IssuedAt      int64  `json:"iat,string"`
	Expiration    int64  `json:"exp,string"`
	JWTID         string `json:"jti"`
	Algorithm     string `json:"alg"`
	KID           string `json:"kid"`
	Type          string `json:"typ"`
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, "login.html")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "need GET or POST", http.StatusBadRequest)
		return
	}

	// POST

	idToken := r.PostFormValue("id_token")
	if idToken == "" {
		http.Error(w, "missing id_token", http.StatusBadRequest)
		return
	}

	// So, like, this endpoint is supposedly just for debugging.
	// But I'm the only user...
	url := tokenVerifyURL + idToken
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get(%s) = error: %v", url, err)
		http.Error(w, "couldn't validate token", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	info := new(tokenVerification)
	if err := json.NewDecoder(resp.Body).Decode(info); err != nil {
		log.Printf("Decode() = error: %v", err)
		http.Error(w, "couldn't validate token", http.StatusInternalServerError)
		return
	}

	if info.Audience != s.site.WebSignInClientID {
		http.Error(w, "wrong aud", http.StatusUnauthorized)
		return
	}
	if !info.EmailVerified || info.Email != s.site.AdminEmail {
		http.Error(w, "you are not the admin", http.StatusUnauthorized)
		return
	}

	// Want to get the session whether or not it already exists or is valid
	sess, _ := s.site.cookieStore.Get(r, "userinfo")
	sess.Values["user_id"] = info.Email
	if err := sess.Save(r, w); err != nil {
		log.Printf("sess.Save(r, w) = error: %v", err)
		http.Error(w, "saving session", http.StatusInternalServerError)
		return
	}

	if redir := r.URL.Query().Get("redirect_to"); redir != "" {
		http.Redirect(w, r, redir, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/edit", http.StatusFound)
}
