VERSION := master
LOCAL_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
DEV_IMAGE_REGISTRY := gcr.io/dcvisor-162009/alcide/dcvisor-dev
BUILDNUM := local
DEVBOX_TERMINAL_OPT := -it
ORIGIN := $(shell git config remote.origin.url)
REPO := github.com/rapid7/csp-cwp-common
REPO_NAME := common
GIT_HASH := $(shell git rev-parse HEAD)
ARTIFACTS_DIR := artifacts

ALCIDE_CGO_CFLAGS :=
ALCIDE_CGO_LDFLAGS :=
ALCIDE_BIN_END_WITH := _strip

#Jenkins builds pass that
ifneq ($(BUILD_NUM),)
	BUILDNUM = $(BUILD_NUM)
endif

ifneq ($(BRANCH_NAME),)
	LOCAL_BRANCH = $(BRANCH_NAME)
endif

ifneq ($(BUILD_VERSION),)
	VERSION = $(BUILD_VERSION)
	DEVBOX_TERMINAL_OPT = -i
endif

ifneq ($(DEBUG),)
	GOFLAGS := -"gcflags='all=-N -l'"
        DLV_CMD := $(GOPATH)/bin/dlv --listen=:2345 --headless=true --api-version=2 exec
        DLV_ARGS := --
        SCONS_DATAPATH_SIM_VAR := ufwk_rtmemchk
        ALCIDE_CGO_CFLAGS := -fsanitize=address -fno-omit-frame-pointer
        ALCIDE_CGO_LDFLAGS := -lasan
        ALCIDE_BIN_END_WITH :=
endif
ALCIDE_CGO_CFLAGS += -fgnu89-inline
ALCIDE_CGO_LDFLAGS += -lpcap -lgcov



ifeq ($(shell uname),Linux)
	LINUX_KERNEL_VERSION ?= $(shell uname -r)
endif


LINUX_KERNEL_VERSION ?= 4.4.0-57-generic
BUILD_LINUX_KERNEL_VERSION ?= 4.4.0-57-generic


DEVBOX_HOME := /home/devbox
DEVBOX_TAG ?= 1.4.0-$(LINUX_KERNEL_VERSION)
DEVBOX_IMAGE := gcr.io/dcvisor-162009/alcide/devbox:goodbye_glide

ifeq ($(shell uname),Linux)
    INSIDE_DEVBOX := $(shell getent passwd devbox)
endif

ifneq (,$(INSIDE_DEVBOX))
	RUN_IN_DEVBOX := /bin/bash
	SUDO := sudo
else
	DEVBOX_GOPATH := $(DEVBOX_HOME)/gopath
	DEVBOX_GRADLE_PATH := $(DEVBOX_HOME)/.gradle
	DEVBOX_REPO := $(DEVBOX_GOPATH)/src/$(REPO)
	SUDO :=

    ifdef GOPATH
        GOPATH_VOLUME := -v $(GOPATH):$(DEVBOX_GOPATH)
    else
        GOPATH_VOLUME :=
    endif

	RUN_IN_DEVBOX := docker run --name devbox --cap-add=ALL --privileged --pid=host --net=host -v /sys/fs:/sys/fs --rm $(DEVBOX_TERMINAL_OPT) $(GOPATH_VOLUME) -v $(PWD):$(DEVBOX_REPO) -v $(HOME)/.kube:$(DEVBOX_HOME)/.kube -v /var/run/docker.sock:/var/run/docker.sock  -v $(HOME)/.gitconfig:$(DEVBOX_HOME)/.gitconfig -v $(HOME)/.ssh:$(DEVBOX_HOME)/.ssh -w $(DEVBOX_REPO) -e LOCAL_USER_ID=`id -u $(USER)` -e LOCAL_GROUP_ID=`id -g $(USER)` -e BUILD_VERSION=$(VERSION) -e GOPRIVATE=github.com/rapid7  $(DEVBOX_IMAGE)
endif

.PHONY: devbox-shell help check-devbox-env

%-in-devbox: check-devbox-env
	$(RUN_IN_DEVBOX) /bin/bash -x -c "make $(@:-in-devbox=)"


check-devbox-env:
ifeq ($(DEVBOX_TAG),1.3.0-)
	$(error Please specify LINUX_KERNEL_VERSION or DEVBOX_TAG)
endif

DEVBOX_TMP := devbox_tmp
devbox-host-bcc: check-devbox-env ##@Misc Setup host bcc
	sudo -E mkdir -p /usr/lib/x86_64-linux-gnu
	docker stop $(DEVBOX_TMP) || true
	docker rm $(DEVBOX_TMP) || true
	docker run -d --name=$(DEVBOX_TMP) $(DEVBOX_IMAGE)
	sudo -E docker cp $(DEVBOX_TMP):/lib/bcc /lib/
	sudo -E docker cp $(DEVBOX_TMP):/usr/include/bcc /usr/include/
	sudo -E docker cp $(DEVBOX_TMP):/usr/lib/x86_64-linux-gnu/libbpf.so /usr/lib/x86_64-linux-gnu/
	sudo -E docker cp $(DEVBOX_TMP):/usr/lib/x86_64-linux-gnu/libbcc.so /usr/lib/x86_64-linux-gnu/
	sudo ln -s -f /usr/lib/x86_64-linux-gnu/libbcc.so /usr/lib/x86_64-linux-gnu/libbcc.so.0
	docker stop $(DEVBOX_TMP)
	docker rm $(DEVBOX_TMP)

devbox-shell: ##@Misc Run a developer sandbox shell. Optionally specify a command line in the variable CMD.
	DEVBOX_TERMINAL=-it
ifeq ($(CMD),)
	$(RUN_IN_DEVBOX)
else
	$(RUN_IN_DEVBOX) /bin/bash -c $(CMD)
endif


define go_build_version_args
	-ldflags '-X $(REPO)/$(1)/version.Version=$(VERSION) -X $(REPO)/$(1)/version.GitHash=$(GIT_HASH)'
endef

define go_build_common_version_args
	-ldflags '-X  github.com/rapid7/csp-cwp-common/pkg/version.Version=$(VERSION)-$(BUILDNUM) -X  github.com/rapid7/csp-cwp-common/pkg/version.GitHash=$(GIT_HASH) -X  github.com/rapid7/csp-cwp-common/pkg/version.AppName=$(1) -X  github.com/rapid7/csp-cwp-common/pkg/tracing.AppName=$(1)'
endef

%-for-linux:
	GOOS=linux $(MAKE) $(@:-for-linux=)

HELP_FUN = \
         %help; \
         while(<>) { push @{$$help{$$2 // 'options'}}, [$$1, $$3] if /^(.+)\s*:.*\#\#(?:@(\w+))?\s(.*)$$/ }; \
         print "Usage: make [options] [target] ...\n\n"; \
    	 for (sort keys %help) { \
         print "$$_:\n"; \
         for (sort { $$a->[0] cmp $$b->[0] } @{$$help{$$_}}) { \
             $$sep = " " x (30 - length $$_->[0]); \
             print "  $$_->[0]$$sep$$_->[1]\n" ; \
         } print "\n"; }

help: ##@Misc Show this help
	@perl -e '$(HELP_FUN)' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

USERID=$(shell id -u)
