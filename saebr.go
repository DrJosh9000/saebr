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

// Package saebr provides a simple blog or CMS for running on App Engine.
package saebr // import "github.com/DrJosh9000/saebr"

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

const cacheTTL = time.Minute

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

type server struct {
	client *datastore.Client
	site   *Site
}

// Run runs saebr.
//
// saebr makes the following assumptions:
//
// * It's running on Google App Engine, so runs as an unencrypted HTTP
//   server. (App Engine can provide HTTPS and HTTP/2.)
// * Serving port is given by the PORT env var, or if empty assumes 8080.
// * The Cloud Datastore to use will be named the same as the GCP project,
//   which is provided by the GOOGLE_CLOUD_PROJECT env var.
// * There is a SITE_KEY env var, which provides the key for the Site
//   datastore entity to use.
// * The Site entity contains all the necessary fields filled in with
//   appropriate data (e.g. Secret contains a suitably random secret,
//   WebSignInClientID is a valid client ID for Google Signin, etc).
//
// TODO: reimplement the above assumptions with functional options instead
// TODO: create a default Site entity when none is found
func Run() {
	ctx := context.Background()
	dscli, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		log.Fatalf("Couldn't create datastore client: %v", err)
	}
	site := &Site{Key: datastore.NameKey("Site", os.Getenv("SITE_KEY"), nil)}
	if err := dscli.Get(ctx, site.Key, site); err != nil {
		log.Fatalf("Couldn't fetch site object: %v", err)
	}
	if len(site.Secret) < 16 {
		log.Fatal("Insufficient secret (len < 16)")
	}
	site.cookieStore = sessions.NewCookieStore([]byte(site.Secret))
	site.pageTmpl = template.Must(template.ParseFiles(site.PageTemplate))
	svr := &server{
		client: dscli,
		site:   site,
	}
	cache := &cache{
		limit: 10000,
		cache: make(map[string]cacheEntry),
		notFound: sitePage{
			site: site,
			page: notFoundPage,
		},
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	r := mux.NewRouter()

	r.Handle("/sitemap.xml", cache.server(svr.fetchSitemap))
	r.Handle("/rss.xml", cache.server(svr.fetchRSS))
	r.Handle("/atom.xml", cache.server(svr.fetchAtom))
	r.Handle("/feed.json", cache.server(svr.fetchJSONFeed))
	r.Handle("/index", cache.server(svr.fetchIndex))
	r.HandleFunc("/login", svr.handleLogin)

	s := r.PathPrefix("/edit").Subrouter()
	s.Use(svr.authMiddleware)
	s.HandleFunc("/{page}", svr.handleEditGet).Methods(http.MethodGet)
	s.HandleFunc("/{page}", svr.handleEditPost).Methods(http.MethodPost)
	s.HandleFunc("", svr.handleEditGet).Methods(http.MethodGet)
	s.HandleFunc("", svr.handleEditPost).Methods(http.MethodPost)

	p := r.PathPrefix("/preview").Subrouter()
	p.Use(svr.authMiddleware)
	p.HandleFunc("/{page}", svr.handlePreview)

	r.Handle("/{page}", cache.server(svr.fetchPage))
	r.Handle("/", cache.server(svr.fetchLatest))

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("http.ListenAndServe: %v", err)
	}
}
