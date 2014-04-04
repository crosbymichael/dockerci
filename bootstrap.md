### post boot

#!/bin/bash

apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y cgroup-lite linux-image-extra-`uname -r`

groupadd docker
usermod -a -G docker ubuntu

apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9
sh -c "echo deb http://get.docker.io/ubuntu docker main > /etc/apt/sources.list.d/docker.list"
apt-get update && apt-get install -y lxc-docker



### docker init script for skydock dns

description "Docker daemon"

start on filesystem
stop on runlevel [!2345]

respawn

pre-start script
        # see also https://github.com/tianon/cgroupfs-mount/blob/master/cgroupfs-mount
        if grep -v '^#' /etc/fstab | grep -q cgroup \
                || [ ! -e /proc/cgroups ] \
                || [ ! -d /sys/fs/cgroup ]; then
                exit 0
        fi
        if ! mountpoint -q /sys/fs/cgroup; then
                mount -t tmpfs -o uid=0,gid=0,mode=0755 cgroup /sys/fs/cgroup
        fi
        (
                cd /sys/fs/cgroup
                for sys in $(awk '!/^#/ { if ($4 == 1) print $1 }' /proc/cgroups); do
                        mkdir -p $sys
                        if ! mountpoint -q $sys; then
                                if ! mount -n -t cgroup -o $sys cgroup $sys; then
                                        rmdir $sys || true
                                fi
                        fi
                done
        )
end script

script
        # modify these in /etc/default/$UPSTART_JOB (/etc/default/docker)
        DOCKER=/usr/bin/$UPSTART_JOB
        DOCKER_OPTS="--dns 172.17.42.1 -D -H unix:///var/run/docker.sock -H tcp://172.17.42.1:4243"
        if [ -f /etc/default/$UPSTART_JOB ]; then
                . /etc/default/$UPSTART_JOB
        fi
        "$DOCKER" -d $DOCKER_OPTS
end script

### finally run start.sh
