FROM crosbymichael/golang

RUN go get -u github.com/crosbymichael/dockerci

ADD . /go/src/github.com/crosbymichael/dockerci
RUN cd /go/src/github.com/crosbymichael/dockerci && go install . ./...
