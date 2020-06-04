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
	"time"

	"github.com/gorilla/mux"
)

func (s *server) handlePreview(w http.ResponseWriter, r *http.Request) {
	ctx, canc := context.WithTimeout(r.Context(), 10*time.Second)
	defer canc()

	sp, err := s.fetchDraftPage(ctx, mux.Vars(r))
	if err != nil {
		log.Printf("not found: %v", err)
	}
	sp.Render(w, r)
}
