version?=0.0.1
pkg=perftest
export GOPATH=$(CURDIR)

flag+=-X main.versionstr=v$(version)$(shell date +.%Y%m%d.%H%M%S)

all: build

deps:
	# go get github.com

fmt:
	go fmt $(pkg)

build: fmt deps
	go build -ldflags "$(flag)" $(pkg)

install: fmt deps
	go install -ldflags "$(flag)" $(pkg)

deploy: build
	# sshpass -p <pass> scp -o StrictHostKeyChecking=no $pkg $url

clean:
	/bin/rm -rf bin $(pkg)
