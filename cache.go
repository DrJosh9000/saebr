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
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type content interface {
	Render(http.ResponseWriter, *http.Request)
}

type cacheEntry struct {
	fetched time.Time
	content content
}

type cache struct {
	limit    int
	cache    map[string]cacheEntry
	mu       sync.RWMutex
	notFound content
}

func (c *cache) get(page string) (cacheEntry, bool) {
	c.mu.RLock()
	ent, ok := c.cache[page]
	c.mu.RUnlock()
	return ent, ok
}

// Random eviction cache.
func (c *cache) put(page string, ent cacheEntry) {
	c.mu.Lock()
	for k := range c.cache {
		if len(c.cache) < c.limit {
			break
		}
		delete(c.cache, k)
	}
	c.cache[page] = ent
	c.mu.Unlock()
}

type fetcherFunc func(context.Context, map[string]string) (content, error)

func (c *cache) server(fetcher fetcherFunc) *cacheServer {
	return &cacheServer{
		cache:   c,
		fetcher: fetcher,
	}
}

type cacheServer struct {
	cache   *cache
	fetcher fetcherFunc
}

func (c *cacheServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if shouldTarpit(r.URL.Path) {
		tarpit(w)
		return
	}

	ctx, canc := context.WithTimeout(r.Context(), 10*time.Second)
	defer canc()

	vars := mux.Vars(r)
	path := r.URL.Path

	// In cache?
	if ent, found := c.cache.get(path); found {
		// Is it fresh enough to serve?
		if ent.fetched.Add(cacheTTL).After(time.Now()) {
			ent.content.Render(w, r)
			return
		}
	}

	cont, err := c.fetcher(ctx, vars)
	if err != nil {
		log.Printf("Couldn't fetch content for %q: %v", path, err)
	}
	if cont == nil {
		cont = c.cache.notFound
	}
	c.cache.put(path, cacheEntry{
		fetched: time.Now(),
		content: cont,
	})
	cont.Render(w, r)
}
