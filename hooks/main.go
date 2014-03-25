// because it does not have to be complicated
package main

import (
	"encoding/json"
	"github.com/bitly/go-nsq"
	"github.com/drone/go-github/github"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
)

var writer *nsq.Writer

// handle pull request events
func pullRequest(w http.ResponseWriter, r *http.Request) {
	hook, err := github.ParsePullRequestHook([]byte(r.FormValue("payload")))
	if err != nil {
		log.Fatal(err)
	}
	switch hook.Action {
	case "open", "synchronize":
		data, err := json.Marshal(hook.PullRequest)
		if err != nil {
			log.Fatal(err)
		}
		if err := writer.PublishAsync("builds", data, nil); err != nil {
			log.Fatal(err)
		}
	default:
		log.Printf("event=%s action=%s\n", r.Header.Get("X-Github-Event"), hook.Action)
	}
}

// handle all other events from the api
func genericEvents(w http.ResponseWriter, r *http.Request) {
	log.Printf("event=%s\n", r.Header.Get("X-Github-Event"))
}

// handle ping events from github api
func ping(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusOK), http.StatusOK)
}

func main() {
	// TODO: set host to filter pushes from random places
	r := mux.NewRouter()

	r.HandleFunc("/", ping).
		Headers("X-Github-Event", "ping").
		Methods("POST")

	r.HandleFunc("/", pullRequest).
		Headers("X-Github-Event", "pull_request").
		Methods("POST")

	r.HandleFunc("/", genericEvents).
		Methods("POST")

	writer = nsq.NewWriter(os.Getenv("NSQD"))
	defer writer.Close()

	if err := http.ListenAndServe(":80", r); err != nil {
		log.Fatal(err)
	}
}
