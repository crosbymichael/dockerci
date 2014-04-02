package dockerci

import (
	"github.com/bitly/go-simplejson"
)

func GetRepoNameAndSha(json *simplejson.Json) (string, string, error) {
	repo, pullrequest := json.Get("repository"), json.Get("pull_request")
	repoName, err := repo.Get("name").String()
	if err != nil {
		return "", "", err
	}
	sha, err := pullrequest.Get("head").Get("sha").String()
	if err != nil {
		return "", "", err
	}
	return repoName, sha, nil
}
