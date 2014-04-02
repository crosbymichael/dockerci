#!/bin/bash

echo 'you need to have skydock running'

docker run --name nsqd1 -d crosbymichael/nsqd -broadcast-address nsqd1.nsqd.prod.docker 
docker run --name nsqadmin1 -d crosbymichael/nsqadmin -nsqd-http-address nsqd.prod.docker:4151

docker run --name redis1 -d crosbymichael/redis

docker run --name hooks1 -d -e REDIS=redis.prod.docker -e NSQD=nsqd.prod.docker crosbymichael/dockerci hooks
echo 'you probably want to put this into hipache'
