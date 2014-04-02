## dockerci

A ci system built with very small apps that run in docker containers.  Each app does one thing and does it well.  Apps can be changed, replaced, and moved around without affecting others.

Workers are able to scale out horizontally across multiple hosts.  To get up and running look at `start.sh` to bootstrap the setup.

#### stack
* docker
* go
* nsq
* skydns
* skydock
* redis
* hipache

This is also a place for others to learn how to setup multi container systems the right way by the maintainers of docker.

#### apps
* hooks - small application that handles github webhooks and pushes jobs on a queue
* worker - app to process jobs via the make file in the docker repository make (binary, test, cross, test-integration)


#### TODO
* add irc bot to query and repo build status
* add github post backs on PRs 
* metrics and stats
* show a multi host setup
* setup nightly builds for docker
* push artifacts to S3 or other dest
* error reporting
