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
	"errors"
	"net/http"

	"cloud.google.com/go/datastore"
)

// Fetches the latest blog post.
func (s *server) fetchLatest(ctx context.Context, _ map[string]string) (content, error) {
	q := datastore.NewQuery("Page").
		Ancestor(s.site.Key).
		FilterField("Published", "=", true).
		FilterField("Blog", "=", true).
		Order("-Created").
		Limit(1)

	var pages []*Page
	if _, err := s.client.GetAll(ctx, q, &pages); err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, errors.New("no pages returned")
	}
	return sitePage{
		site: s.site,
		page: pages[0],
	}, nil
}

// Redirects to the latest blog post.
func (s *server) redirectToLatest(w http.ResponseWriter, r *http.Request) {
	latest, err := s.fetchLatest(r.Context(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	sp := latest.(sitePage)
	http.Redirect(w, r, "/"+sp.page.Key.Name, http.StatusFound)
}
