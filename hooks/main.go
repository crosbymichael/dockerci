package main

import (
	"github.com/bitly/go-simplejson"
	"github.com/crosbymichael/dockerci/datastore"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
)

// handle pull request events
func pullRequest(w http.ResponseWriter, r *http.Request) {
	store, err := datastore.New(os.Getenv("REDIS_ADDR"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	defer r.Body.Close()

	blob, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	data, err := simplejson.NewJson(blob)
	if err != nil {
		log.Fatal(err)
	}

	i, err := data.Get("Number").Int()
	if err != nil {
		log.Fatal(err)
	}

	if err := store.SavePullRequest(getRepositoryName(r), strconv.Itoa(i), blob); err != nil {
		log.Fatal(err)
	}
}

// handle all other events from the api to track notifications
// in the future
//
// we will just log the even for now
func genericEvents(w http.ResponseWriter, r *http.Request) {
	log.Printf("event=%s repository=%s\n", r.Header.Get("X-Github-Event"), getRepositoryName(r))
}

// handle ping events from github api
func ping(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusOK), http.StatusOK)
}

func getRepositoryName(r *http.Request) string {
	return mux.Vars(r)["name"]
}

func main() {
	// TODO: set host to filter pushes from random places
	r := mux.NewRouter()

	r.HandleFunc("/{name:.*}/", ping).
		Headers("X-Github-Event", "ping").
		Methods("POST")

	r.HandleFunc("/{name:.*}/", pullRequest).
		Headers("X-Github-Event", "pull_request").
		Methods("POST")

	r.HandleFunc("/{name:.*}/", genericEvents).
		Methods("POST")

	if err := http.ListenAndServe(":80", r); err != nil {
		log.Fatal(err)
	}
}
