FROM crosbymichael/golang

RUN apt-get update && apt-get install -y mercurial make

RUN go get -d github.com/crosbymichael/dockerci && \
    go get github.com/bitly/go-nsq && \
    go get github.com/drone/go-github/github && \
    go get github.com/gorilla/mux

ADD . /go/src/github.com/crosbymichael/dockerci
RUN cd /go/src/github.com/crosbymichael/dockerci && go install . ./...
ENV PATH $PATH:/go/bin

# make git happy
RUN git config --global user.name dockerci && \
    git config --global user.email dockerci@example.com && \
    ln -s /.dockerinit /usr/bin/docker
