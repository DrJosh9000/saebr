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
	"crypto/rand"
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
	client  *datastore.Client
	site    *Site
	options *options
}

type options struct {
	cacheMaxSize int
	dsProjectID  string
}

// Option is the type of each functional option to Run.
type Option func(*options)

// TODO: provide an option for disabling the cache.

// CacheMaxSize configures the maximum size for the page cache. The default is
// 10000.
func CacheMaxSize(n int) Option {
	return func(o *options) { o.cacheMaxSize = n }
}

// DatastoreProjectID sets the project ID used for the Cloud Datastore client.
// The default is the empty string (the client then obtains the project ID from
// the DATASTORE_PROJECT_ID env var).
func DatastoreProjectID(projID string) Option {
	return func(o *options) { o.dsProjectID = projID }
}

// Run runs saebr.
//
// saebr makes the following assumptions:
//
// * It's running on Google App Engine, so runs as an unencrypted HTTP
//   server. (App Engine can provide HTTPS and HTTP/2.)
// * Run can exit the program (using log.Fatal) if an error occurs.
// * Serving port is given by the PORT env var, or if empty assumes 8080.
func Run(siteKey string, opts ...Option) {
	ctx := context.Background()

	o := &options{
		cacheMaxSize: 10000,
	}
	for _, opt := range opts {
		opt(o)
	}

	dscli, err := datastore.NewClient(ctx, o.dsProjectID)
	if err != nil {
		log.Fatalf("Couldn't create datastore client: %v", err)
	}
	site := &Site{Key: datastore.NameKey("Site", siteKey, nil)}
	if err := dscli.Get(ctx, site.Key, site); err != nil {
		if err != datastore.ErrNoSuchEntity {
			log.Fatalf("Couldn't fetch site object: %v", err)
		}
		// Fill in some sensible defaults and create it
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Fatalf("Couldn't generate a secret: %v", err)
		}
		site.Secret = string(secret)
		site.AdminEmail = "your.google.account.email.address@example.com"
		site.FeedAuthor = "Your Name"
		site.FeedCopyright = "Copyright © Your Name"
		site.FeedDescription = "Description for feeds"
		site.FeedSubtitle = "Subtitle for feeds"
		site.FeedTitle = "Title for feeds"
		site.PageTemplate = "your_page_template.html"
		site.TimeLocation = time.Local.String()
		site.URLBase = "https://your.site.example.com/"
		site.WebSignInClientID = "a web sign-in client ID - typically a number, then some base64 encoded data, followed by .apps.googleusercontent.com"
		if _, err := dscli.Put(ctx, site.Key, site); err != nil {
			log.Fatalf("Couldn't create a new Site entity: %v", err)
		}
	}
	if len(site.Secret) < 16 {
		log.Fatal("Insufficient secret (len < 16)")
	}
	loc, err := time.LoadLocation(site.TimeLocation)
	if err != nil {
		log.Fatalf("Couldn't load time location: %v", err)
	}
	site.timeLoc = loc
	fi, err := os.Stat(site.PageTemplate)
	if err != nil {
		log.Fatalf("Couldn't find page template: %v", err)
	}
	site.pageTmplMtime = fi.ModTime()
	site.cookieStore = sessions.NewCookieStore([]byte(site.Secret))
	site.pageTmpl = template.Must(template.ParseFiles(site.PageTemplate))
	svr := &server{
		client:  dscli,
		site:    site,
		options: o,
	}
	cache := &cache{
		limit: o.cacheMaxSize,
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
