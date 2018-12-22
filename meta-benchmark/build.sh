#!/bin/bash
version=${1:-0.0.1}
pkg=perftest
CURDIR=$(cd $(dirname $0); pwd)
export GOPATH=$CURDIR
go fmt $pkg
go build -ldflags "-X main.versionstr=v$version$(date +.%Y%m%d.%H%M%S)" $pkg
# upload
# sshpass -p <pass> scp -o StrictHostKeyChecking=no $pkg $url
