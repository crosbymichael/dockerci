package dockerci

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"log"
	"os/exec"
	"time"
)

type Result struct {
	Success bool
	Output  string
	Method  string
}

func (r *Result) ToData() map[string]string {
	var (
		stateKey = fmt.Sprintf("method-%s", r.Method)
		data     = map[string]string{
			fmt.Sprintf("%s-output", stateKey): r.Output,
			fmt.Sprintf("%s-result", stateKey): "failed",
		}
	)

	if r.Success {
		data[fmt.Sprintf("%s-result", stateKey)] = "passed"
	}
	return data
}

func GetSha(json *simplejson.Json) (string, error) {
	sha, err := json.Get("pull_request").Get("head").Get("sha").String()
	if err != nil {
		return "", err
	}
	return sha, nil
}

func Checkout(temp string, json *simplejson.Json) error {
	// git clone -qb master https://github.com/upstream/docker.git our-temp-directory
	base := json.Get("base")
	ref, err := base.Get("ref").String()
	if err != nil {
		return err
	}
	url, err := base.Get("repo").Get("clone_url").String()
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "clone", "-qb", ref, url, temp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		return err
	}

	head := json.Get("head")
	url, err = head.Get("repo").Get("clone_url").String()
	if err != nil {
		return err
	}
	ref, err = head.Get("ref").String()
	if err != nil {
		return err
	}
	log.Printf("ref=%s url=%s\n", ref, url)
	// cd our-temp-directory && git pull -q https://github.com/some-user/docker.git some-feature-branch
	cmd = exec.Command("git", "pull", "-q", url, ref)
	cmd.Dir = temp
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		return err
	}
	return nil
}

func Build(temp, name string) error {
	cmd := exec.Command("docker", "build", "-t", name, ".")
	cmd.Dir = temp

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		return err
	}
	return nil
}

func MakeTest(temp, method, name string) (*Result, error) {
	var (
		result = &Result{Method: method}
		cmd    = exec.Command("docker", "run", "-t", "--privileged", "--name", name, "docker", "hack/make.sh", method)
	)
	cmd.Dir = temp

	output, err := cmd.CombinedOutput()
	if err != nil {
		// it's ok for the make command to return a non-zero exit
		// incase of a failed build
		if _, ok := err.(*exec.ExitError); !ok {
			log.Println(string(output))
			return nil, err
		}
	} else {
		result.Success = true
	}
	result.Output = string(output)

	return result, nil
}

func LogTime(store *Store, queue string, started time.Time) {
	duration := time.Now().Sub(started)
	store.SaveMessageDuration(queue, duration.Seconds())
}
