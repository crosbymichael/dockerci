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

type runData struct {
	output []byte
	err    error
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

func MakeTest(temp, method, image, name string, duration time.Duration) (*Result, error) {
	var (
		c      = make(chan *runData)
		result = &Result{Method: method}
		cmd    = exec.Command("docker", "run", "-t", "--privileged", "--name", name, image, "hack/make.sh", method)
	)
	cmd.Dir = temp

	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			// it's ok for the make command to return a non-zero exit
			// incase of a failed build
			if _, ok := err.(*exec.ExitError); !ok {
				log.Println(string(output))
			} else {
				err = nil
			}
		}
		c <- &runData{output, err}
	}()

	select {
	case data := <-c:
		if data.err != nil {
			return nil, data.err
		}
		result.Success = true
		result.Output = string(data.output)
	case <-time.After(duration):
		if err := cmd.Process.Kill(); err != nil {
			log.Println(err)
		}
		return nil, fmt.Errorf("killed because build took to long")
	}
	return result, nil
}

func LogTime(store *Store, queue string, started time.Time) {
	duration := time.Now().Sub(started)
	store.SaveMessageDuration(queue, duration.Seconds())
}
