package main

import (
	"encoding/json"
	"fmt"
	"github.com/bitly/go-nsq"
	"github.com/drone/go-github/github"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type handler struct {
}

func (h *handler) HandleMessage(msg *nsq.Message) error {
	var pr *github.PullRequest
	if err := json.Unmarshal(msg.Body, &pr); err != nil {
		return err
	}

	// checkout the code in a temp dir
	temp, err := ioutil.TempDir("", fmt.Sprintf("pr-%d", pr.Number))
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	if err := checkout(temp, pr); err != nil {
		return err
	}

	// run make test-integration
	output, err := makeTest(temp)
	if err != nil {
		return err
	}

	if err := pushResults(pr, output); err != nil {
		return err
	}
	return nil
}

func checkout(temp string, pr *github.PullRequest) error {
	// git clone -qb master https://github.com/upstream/docker.git our-temp-directory
	cmd := exec.Command("git", "clone", "-qb", pr.Base.Ref, pr.Base.Repo.Url, temp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		return err
	}

	// cd our-temp-directory && git pull -q https://github.com/some-user/docker.git some-feature-branch
	cmd = exec.Command("git", "pull", "-q", pr.Head.Repo.Url, pr.Head.Ref)
	cmd.Dir = temp
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		return err
	}
	return nil
}

func makeTest(temp string) ([]byte, error) {
	cmd := exec.Command("make", "binary") // just testing binary for now
	cmd.Dir = temp

	output, err := cmd.CombinedOutput()
	if err != nil {
		// it's ok for the make command to return a non-zero exit
		// incase of a failed build
		if _, ok := err.(*exec.ExitError); !ok {
			return output, err
		}
	}
	return output, nil
}

func pushResults(pr *github.PullRequest, output []byte) error {
	return nil
}

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	reader, err := nsq.NewReader("builds", "binary")
	if err != nil {
		log.Fatal(err)
	}
	reader.AddHandler(&handler{})

	if err := reader.ConnectToNSQ(os.Getenv("NSQD")); err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case <-reader.ExitChan:
			return
		case <-sigChan:
			// if we receive a sig then stop the reader
			reader.Stop()
		}
	}
}
