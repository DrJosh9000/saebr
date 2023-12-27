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
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/feeds"
)

func (s *server) fetchFeed(ctx context.Context) (*feeds.Feed, error) {
	q := datastore.NewQuery("Page").
		Ancestor(s.site.Key).
		FilterField("Published", "=", true).
		FilterField("Blog", "=", true).
		Order("-Created")

	var pages []*Page
	if _, err := s.client.GetAll(ctx, q, &pages); err != nil {
		return nil, fmt.Errorf("fetching all posts: %v", err)
	}

	author := &feeds.Author{Name: s.site.FeedAuthor}
	feed := &feeds.Feed{
		Title:       s.site.FeedTitle,
		Subtitle:    s.site.FeedSubtitle,
		Link:        &feeds.Link{Href: s.site.URLBase},
		Description: s.site.FeedDescription,
		Author:      author,
		Copyright:   s.site.FeedCopyright,
	}

	for _, page := range pages {
		if page.Created.After(feed.Updated) {
			feed.Updated = page.Created
		}
		if page.LastModified.After(feed.Updated) {
			feed.Updated = page.LastModified
		}
		link := s.site.URLBase + page.Key.Name
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       page.Title,
			Link:        &feeds.Link{Href: link},
			Author:      author,
			Id:          link,
			Updated:     page.LastModified,
			Created:     page.Created,
			Content:     string(blackfridayRun(page.Contents)),
			Description: page.Description,
		})
	}
	return feed, nil
}

type feedContent struct {
	contentType string
	updated     time.Time
	method      func() (string, error)
}

func (c *feedContent) Render(w http.ResponseWriter, r *http.Request) {
	x, err := c.method()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", c.contentType)
	http.ServeContent(w, r, strings.TrimPrefix(r.URL.Path, "/"), c.updated, strings.NewReader(x))
}

func (s *server) fetchRSS(ctx context.Context, _ map[string]string) (content, error) {
	feed, err := s.fetchFeed(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching feed: %v", err)
	}
	return &feedContent{
		contentType: "application/rss+xml",
		method:      feed.ToRss,
		updated:     feed.Updated,
	}, nil
}

func (s *server) fetchAtom(ctx context.Context, _ map[string]string) (content, error) {
	feed, err := s.fetchFeed(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching feed: %v", err)
	}
	return &feedContent{
		contentType: "application/atom+xml",
		method:      feed.ToAtom,
		updated:     feed.Updated,
	}, nil
}

func (s *server) fetchJSONFeed(ctx context.Context, _ map[string]string) (content, error) {
	feed, err := s.fetchFeed(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching feed: %v", err)
	}
	return &feedContent{
		contentType: "application/json",
		method:      feed.ToJSON,
		updated:     feed.Updated,
	}, nil
}
