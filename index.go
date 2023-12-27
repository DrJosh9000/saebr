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
	"strings"
	"text/template"

	"cloud.google.com/go/datastore"
)

var (
	indexTmpl = template.Must(template.New("index.md").Parse(`All blog posts, in reverse chronological order.

{{range .}}
#### {{.Header}}
{{range .Pages}}
*   [{{.Title}}](/{{.Key.Name}}){{if .Edited}} (edited {{.LastModified.Format "January 2006"}}){{end}}{{if .Description}}
	{{.Description}}{{end}}{{end}}

{{end}}`))
)

func (s *server) fetchIndex(ctx context.Context, _ map[string]string) (content, error) {
	q := datastore.NewQuery("Page").
		Ancestor(s.site.Key).
		FilterField("Published", "=", true).
		FilterField("Blog", "=", true).
		Order("-Created").
		Project("Title", "Description", "Created", "LastModified")

	var pages []*Page
	if _, err := s.client.GetAll(ctx, q, &pages); err != nil {
		return nil, fmt.Errorf("fetching all posts: %v", err)
	}
	if len(pages) == 0 {
		return sitePage{
			site: s.site,
			page: &Page{
				Key:         datastore.NameKey("Page", "index", s.site.Key),
				Title:       "Index",
				Published:   true,
				Contents:    "No posts yet!",
				Description: "List of all blog posts, in reverse chronological order.",
			},
		}, nil
	}
	mtime := pages[0].LastModified
	type pageGroup struct {
		Header string
		Pages  []*Page
	}
	g := &pageGroup{
		Header: pages[0].Created.Format("January 2006"),
		Pages:  pages,
	}
	groups := []*pageGroup{g}
	i := 0
	for j, page := range pages {
		mtime = maxTime(mtime, page.LastModified)
		h := page.Created.Format("January 2006")
		if h == g.Header {
			continue
		}
		g.Pages = pages[i:j]
		g = &pageGroup{Header: h, Pages: pages[j:]}
		groups = append(groups, g)
		i = j
	}

	b := new(strings.Builder)
	if err := indexTmpl.Execute(b, groups); err != nil {
		return nil, fmt.Errorf("execute index template: %v", err)
	}
	return sitePage{
		site: s.site,
		page: &Page{
			Key:          datastore.NameKey("Page", "index", s.site.Key),
			Title:        "Index",
			Published:    true,
			LastModified: mtime,
			Description:  "List of all blog posts, in reverse chronological order.",
			Contents:     b.String(),
		},
	}, nil
}
