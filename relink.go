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

	"cloud.google.com/go/datastore"
)

// Checks and relinks all Prev/Next keys.
func (s *server) relink(ctx context.Context) error {
	_, err := s.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery("Page").
			Ancestor(s.site.Key).
			Filter("Published =", true).
			Filter("Blog =", true).
			Order("Created").
			Transaction(tx)

		var pages []*Page
		if _, err := s.client.GetAll(ctx, q, &pages); err != nil {
			return err
		}
		N := len(pages)
		if N == 0 {
			return nil
		}

		// Set of slice indexes that were wrong, to write back
		upd := make(map[int]struct{})

		// First page must have Prev = nil
		if pages[0].Prev != nil {
			pages[0].Prev = nil
			upd[0] = struct{}{}
		}
		// Last page must have Next = nil
		if pages[N-1].Next != nil {
			pages[N-1].Next = nil
			upd[N-1] = struct{}{}
		}
		// Then set prev/next for the middle
		for i := range pages[:N-1] {
			if k := pages[i].Key; !pages[i+1].Prev.Equal(k) {
				pages[i+1].Prev = k
				upd[i+1] = struct{}{}
			}
			if k := pages[i+1].Key; !pages[i].Next.Equal(k) {
				pages[i].Next = k
				upd[i] = struct{}{}
			}
		}
		// Put all changed pages.
		for i := range upd {
			if _, err := tx.Put(pages[i].Key, pages[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
