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
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"golang.org/x/net/xsrftoken"

	"github.com/gorilla/mux"
)

var editTmpl = template.Must(template.New("edit.html").Parse(`<!DOCTYPE html>
<html>

<head>
	<title>Edit</title>
	<link rel="shortcut icon" href="/favicon.ico">
	<link rel="stylesheet" href="https://fonts.googleapis.com/icon?family=Material+Icons">
	<link rel="stylesheet" type="text/css" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/css/materialize.min.css" media="screen,projection" />
	<meta name="viewport" content="width=device-width, initial-scale=1.0" />
</head>

<body>
	<header class="section light-blue darken-1">
		<div class="container">
			<h3 class="white-text">Edit</h3>
		</div>
	</header>
	<article class="section">
		<div class="container">
			<div class="row">
				<form method="POST" id="editform" class="col s12">
					<input type="hidden" name="XSRFToken" value="{{.XSRFToken}}">
				{{with .Page}}
					<div class="input-field col s12">
						<input type="text" name="Key"{{if .Key}} disabled value="{{.Key.Name}}"{{end}}>
						<label for="Key"{{if .Key}} class="active"{{end}}>Key</label>
					</div>
					<div class="input-field col s12">
						<input type="text" name="Title" value="{{.Title}}">
						<label for="Title"{{if .Title}} class="active"{{end}}>Title</label>
					</div>
					<div class="col l3 m6 s12">
						<label>
							<input type="checkbox" class="filled-in" name="Published"{{if .Published}} checked="checked"{{end}}>
							<span>Published</span>
						</label>
					</div>
					<div class="col l3 m6 s12">
						<label>
							<input type="checkbox" class="filled-in" name="Blog"{{if .Blog}} checked="checked"{{end}}>
							<span>Blog</span>
						</label>
					</div>
					<div class="input-field col s12">
						<input type="text" name="Category" value="{{.Category}}">
						<label for="Category"{{if .Category}} class="active"{{end}}>Category</label>
					</div>
					<div class="input-field col s12">
						<input type="text" name="Tags" value="{{.TagList}}">
						<label for="Tags"{{if .Tags}} class="active"{{end}}>Tags</label>
						<span class="helper-text">Comma-separated tag list</span>
					</div>
					<div class="input-field col s12">
						<div id="editor"></div>
						<input type="hidden" id="contents" name="Contents" value="{{.Contents}}">
					</div>
					<div class="col s12">
						{{if .Key}}<a class="btn waves-effect waves-light" href="/preview/{{.Key.Name}}">Preview
							<i class="material-icons right">pageview</i>
						</a>{{end}}
						<button class="btn waves-effect waves-light" type="submit" name="action">Save
							<i class="material-icons right">save</i>
						</button>
					</div>
				{{end}}
				</form>
			</div>
		</div>
	</article>
	<script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/js/materialize.min.js"></script>
	<script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/ace/1.4.12/ace.min.js" charset="utf-8"></script>
	<script>
		ace.config.set("basePath", "https://cdnjs.cloudflare.com/ajax/libs/ace/1.4.12/");
		const editor = ace.edit("editor", {
			mode: "ace/mode/markdown",
			theme: "ace/theme/monokai",
		});

		const contents = document.getElementById("contents");
		editor.session.setValue(contents.value); 
		const form = document.getElementById("editform");
		form.addEventListener("submit", () => { contents.value = editor.session.getValue() });
	</script>
</body>
	
</html>`))

type userIDCtxKey struct{}

type editPage struct {
	XSRFToken string
	Page      *Page
}

func loginRedirect(to *url.URL) string {
	q := make(url.Values)
	q.Set("redirect_to", to.String())
	u := url.URL{
		Path:     "/login",
		RawQuery: q.Encode(),
	}
	return u.String()
}

func tags(list string) []string {
	s := strings.Split(list, ",")
	for i, t := range s {
		s[i] = strings.TrimSpace(t)
	}
	return s
}

func (s *server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := s.site.cookieStore.Get(r, "userinfo")
		if err != nil {
			log.Printf("cookieStore.Get(user) = error: %v", err)
			http.Redirect(w, r, loginRedirect(r.URL), http.StatusFound)
			return
		}
		userID, _ := sess.Values["user_id"].(string)
		if userID != s.site.AdminEmail {
			http.Redirect(w, r, loginRedirect(r.URL), http.StatusFound)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), userIDCtxKey{}, userID))
		next.ServeHTTP(w, r)
	})
}

func userID(ctx context.Context) string {
	return ctx.Value(userIDCtxKey{}).(string)
}

func (s *server) handleEditPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := userID(ctx)
	pkey := mux.Vars(r)["page"]
	if !xsrftoken.Valid(r.PostFormValue("XSRFToken"), s.site.Secret, userID, "edit/"+pkey) {
		http.Error(w, "bad XSRFToken", http.StatusBadRequest)
		return
	}

	nkey := r.PostFormValue("Key")
	title := r.PostFormValue("Title")
	contents := strings.Replace(r.PostFormValue("Contents"), "\r\n", "\n", -1) // see https://github.com/russross/blackfriday/issues/423

	// Form doesn't post disabled field. Deduce from URL.
	if nkey == "" {
		nkey = pkey
	}

	switch "" {
	case nkey:
		http.Error(w, "Key required", http.StatusBadRequest)
		return
	case title:
		http.Error(w, "Title required", http.StatusBadRequest)
		return
	case contents:
		http.Error(w, "Contents required", http.StatusBadRequest)
		return
	}
	key := datastore.NameKey("Page", nkey, s.site.Key)
	page := &Page{Key: key}
	if err := s.client.Get(ctx, key, page); err != nil {
		if err != datastore.ErrNoSuchEntity {
			http.Error(w, "couldn't check for existing entity", http.StatusInternalServerError)
			return
		}
	}
	page.Key = key
	page.Title = title
	page.Contents = contents
	page.Published = r.PostFormValue("Published") == "on"
	page.Blog = r.PostFormValue("Blog") == "on"
	page.Category = r.PostFormValue("Category")
	page.Tags = tags(r.PostFormValue("Tags"))
	page.LastModified = time.Now().In(s.site.timeLoc)
	if page.Created.IsZero() && page.Published {
		page.Created = page.LastModified
	}

	if _, err := s.client.Put(ctx, key, page); err != nil {
		http.Error(w, "couldn't save entity", http.StatusInternalServerError)
		return
	}
	if pkey != nkey {
		http.Redirect(w, r, "/edit/"+nkey, http.StatusFound)
		return
	}
	ed := &editPage{
		XSRFToken: xsrftoken.Generate(s.site.Secret, userID, "edit/"+nkey),
		Page:      page,
	}
	if err := editTmpl.Execute(w, ed); err != nil {
		log.Printf("Couldn't execute editTmpl: %v", err)
	}
}

func (s *server) handleEditGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pkey := mux.Vars(r)["page"]
	ed := &editPage{
		XSRFToken: xsrftoken.Generate(s.site.Secret, userID(ctx), "edit/"+pkey),
		Page: &Page{
			Blog: true,
		},
	}
	if pkey != "" {
		ed.Page.Key = datastore.NameKey("Page", pkey, s.site.Key)
		if err := s.client.Get(ctx, ed.Page.Key, ed.Page); err != nil {
			// Maybe I want to create such a page?
			log.Printf("%q not found: %v", pkey, err)
		}
	}

	if err := editTmpl.Execute(w, ed); err != nil {
		log.Printf("Couldn't execute editTmpl: %v", err)
	}
}
