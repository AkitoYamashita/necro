#!/usr/bin/make -f
SHELL=/usr/bin/env bash
.PHONY: readme
## var
BASE:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
DATEID:=$(shell date +%Y%m%d%H%M%S)
VERSION:=v0.1.0
## README
.DEFAULT_GOAL := readme
define README
--- Makefile Task List --- #COMMENT
endef
export README
readme: # @hide
	@echo ${README}
	@printf "\033[1;31m==== [Makefile] ====\033[0m\n"
	@grep -E '^[^#[:space:]].*:[[:space:]]*(#.*)?$$' Makefile | awk '/@hide$$/ {next} {sub(/#.*/, "\033[90m&\033[0m")}1'
	#@if [ -f other/Makefile ]; then echo; echo "[other/Makefile]"; grep -E '^[^#[:space:]].*:[[:space:]]*(#.*)?$$' other/Makefile; fi
sso_login:
	aws sso login --profile COM_PRD
build_win:
	@GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=dev -X main.commit=local -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o dist/necro_windows_amd64.exe .
build_mac:
	@go build -ldflags "-s -w -X main.version=dev -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o dist/necro .
build: build_mac clean run
clean: ## keep latest 5 log/*.txt and remove older ones
	ls -1t log/*.txt 2>/dev/null | tail -n +6 | xargs -r rm -f
run:
	./dist/necro conf/task.yml
release: ## create git tag and push
	git tag $(VERSION)
	git push origin $(VERSION)
unrelease: ## delete git tag and push
	git tag -d $(VERSION)
	git push origin :refs/tags/$(VERSION)