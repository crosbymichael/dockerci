#!/bin/bash

echo 'you need to have skydock running with env as "prod"'

docker run --name nsqd1 -d crosbymichael/nsqd -broadcast-address nsqd1.nsqd.prod.docker 
docker run --name nsqadmin1 -d crosbymichael/nsqadmin -nsqd-http-address nsqd.prod.docker:4151

docker run --name redis1 -d crosbymichael/redis

docker run --name hooks1 -d -e REDIS=redis.prod.docker:6379 -e NSQD=nsqd.prod.docker:4150 crosbymichael/dockerci hooks
echo 'you probably want to put this into hipache'

# start a worker
# docker run -v /var/run/docker.sock:/var/run/docker.sock -e NSQD=nsqd.prod.docker:4150 crosbymichael/dockerci integration

