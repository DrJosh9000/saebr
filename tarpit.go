package saebr

import (
	"net/http"
	"strings"
	"time"
)

var tarpitSuffixes = []string{
	"/wp-login.php",
	"/wlwmanifest.xml",
	"/xmlrpc.php",
}

func shouldTarpit(path string) bool {
	for _, suf := range tarpitSuffixes {
		if strings.HasSuffix(path, suf) {
			return true
		}
	}
	return false
}

func tarpit(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Length", "9812375982374960220027029911616636350017")
	h.Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-timeout:
			return
		case <-time.After(100 * time.Millisecond):
			if _, err := w.Write([]byte("<!doctype html><html><head><title>nope</title></head><body><pre>")); err != nil {
				return
			}
		}
	}
}
