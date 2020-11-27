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
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
)

var notFoundPage = &Page{
	Key:      datastore.NameKey("Page", "notfound", nil),
	Title:    "Error 404",
	Contents: "#### That URL makes no sense to me\n\n###### Sorry\n\nYou might want to click one of the menu items above, or check the URL and try again.",
}

// Page is the type of each blog post or page.
type Page struct {
	Key          *datastore.Key `datastore:"__key__"`
	Title        string
	Created      time.Time
	LastModified time.Time
	Published    bool
	Blog         bool
	Category     string
	Tags         []string
	Contents     string         `datastore:",noindex"`
	Prev, Next   *datastore.Key `datastore:",noindex"`

	fullHTML string    `datastore:"-"` // Set by Render
	render   sync.Once `datastore:"-"`
}

// Edited reports if the created and last-modified timestamps are different
// by more than 12 hours.
func (p *Page) Edited() bool {
	return p.LastModified.Sub(p.Created) > 12*time.Hour
}

// Latest reports if the page is the latest (i.e. Next is nil).
func (p *Page) Latest() bool {
	return p.Next == nil
}

// TagList returns Tags as a single comma-delimited string.
func (p *Page) TagList() string {
	return strings.Join(p.Tags, ", ")
}

// ContentsHTML translates the Contents from Markdown into HTML, and returns it.
// (You don't have to store Markdown in the Contents field, and you don't have
// to use this method in your template.)
func (p *Page) ContentsHTML() template.HTML {
	return materializeULTags(blackfridayRun(p.Contents))
}

type sitePage struct {
	site *Site
	page *Page
}

// Render renders a page.
func (sp sitePage) Render(w http.ResponseWriter, r *http.Request) {
	if sp.page == nil {
		sp.page = notFoundPage
	}
	sp.page.render.Do(func() {
		b := new(strings.Builder)
		if err := sp.site.pageTmpl.Execute(b, sp.page); err != nil {
			log.Printf("Couldn't execute template: %v", err)
		}
		sp.page.fullHTML = b.String()
	})
	if sp.page.fullHTML == "" {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if sp.page == notFoundPage {
		w.Header().Set("Content-Length", strconv.Itoa(len(sp.page.fullHTML)))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(sp.page.fullHTML))
		return
	}
	http.ServeContent(w, r, "f.html", maxTime(sp.page.LastModified, sp.site.pageTmplMtime), strings.NewReader(sp.page.fullHTML))
}

func (s *server) fetchPage(ctx context.Context, vars map[string]string) (content, error) {
	page := vars["page"]
	key := datastore.NameKey("Page", page, s.site.Key)
	p := new(Page)
	if err := s.client.Get(ctx, key, p); err != nil {
		return nil, fmt.Errorf("get %q from Datastore: %v", page, err)
	}
	if !p.Published {
		return nil, fmt.Errorf("%q not published", page)
	}
	return sitePage{site: s.site, page: p}, nil
}

func (s *server) fetchDraftPage(ctx context.Context, vars map[string]string) (content, error) {
	page := vars["page"]
	key := datastore.NameKey("Page", page, s.site.Key)
	p := new(Page)
	if err := s.client.Get(ctx, key, p); err != nil {
		return nil, fmt.Errorf("get %q from Datastore: %v", page, err)
	}
	return sitePage{site: s.site, page: p}, nil
}
