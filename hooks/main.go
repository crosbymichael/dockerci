package main

import (
	"github.com/bitly/go-nsq"
	"github.com/bitly/go-simplejson"
	"github.com/crosbymichael/dockerci"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
)

var (
	writer *nsq.Writer
	store  *dockerci.Store
)

func pullRequest(w http.ResponseWriter, r *http.Request) {
	rawPayload := []byte(r.FormValue("payload"))
	json, err := simplejson.NewJson(rawPayload)
	if err != nil {
		log.Fatal(err)
	}
	action, err := json.Get("action").String()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("event=%s action=%s\n", r.Header.Get("X-Github-Event"), action)
	switch action {
	case "opened", "synchronize":
		// check that the commit for this PR is not already in the queue or processed
		repoName, sha, err := dockerci.GetRepoNameAndSha(json)
		if err != nil {
			log.Fatal(err)
		}
		if err := store.AtomicSaveState(repoName, sha, "pending"); err != nil {
			if err == dockerci.ErrKeyIsAlreadySet {
				return
			}
			log.Fatal(err)
		}

		if err := writer.PublishAsync("builds", rawPayload, nil); err != nil {
			log.Fatal(err)
		}
	}
}

func ping(w http.ResponseWriter, r *http.Request) {
	log.Printf("event=ping\n")
	http.Error(w, http.StatusText(http.StatusOK), http.StatusOK)
}

func newRouter() *mux.Router {
	r := mux.NewRouter()
	r.Host("api.github.com")
	r.HandleFunc("/", ping).Headers("X-Github-Event", "ping").Methods("POST")
	r.HandleFunc("/", pullRequest).Headers("X-Github-Event", "pull_request").Methods("POST")
	return r
}

func main() {
	writer = nsq.NewWriter(os.Getenv("NSQD"))
	store = dockerci.New(os.Getenv("REDIS"), os.Getenv("REDIS_AUTH"))
	defer store.Close()

	if err := http.ListenAndServe(":80", newRouter()); err != nil {
		log.Fatal(err)
	}
}
