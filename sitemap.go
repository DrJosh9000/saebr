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
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
)

var sitemapTmpl = template.Must(template.New("sitemap.xml").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url>
		<loc>{{$.URLBase}}</loc>
		<lastmod>{{$.Updated.Format "2006-01-02"}}</lastmod>
		<changefreq>weekly</changefreq>
		<priority>1.0</priority>
	</url>
{{- range $.Pages}}
	<url>
		<loc>{{$.URLBase}}{{if ne .Key.Name "default"}}{{.Key.Name}}{{end}}</loc>
		<lastmod>{{.LastModified.Format "2006-01-02"}}</lastmod>
	</url>
{{- end}}
</urlset>`))

func (s *server) fetchSitemap(ctx context.Context, _ map[string]string) (content, error) {
	q := datastore.NewQuery("Page").
		Ancestor(s.site.Key).
		FilterField("Published", "=", true).
		Project("Created", "LastModified")

	var pages []*Page
	if _, err := s.client.GetAll(ctx, q, &pages); err != nil {
		return nil, err
	}

	var lastMod time.Time
	for _, p := range pages {
		if p.Created.After(lastMod) {
			lastMod = p.Created
		}
		if p.LastModified.After(lastMod) {
			lastMod = p.LastModified
		}
	}

	render := func() (string, error) {
		data := &struct {
			URLBase string
			Pages   []*Page
			Updated time.Time
		}{
			URLBase: s.site.URLBase,
			Pages:   pages,
			Updated: lastMod,
		}
		b := new(strings.Builder)
		if err := sitemapTmpl.Execute(b, data); err != nil {
			return "", err
		}
		return b.String(), nil
	}

	return &feedContent{
		contentType: "application/xml",
		updated:     lastMod,
		method:      render,
	}, nil

}
