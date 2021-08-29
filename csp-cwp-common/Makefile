include build/definitions.mk

.SECONDARY:
.SECONDEXPANSION:
.PHONY: proto

all: proto go-format go-lint test ##@Build Build the code tree

PROTO_DIRS := $(wildcard pkg/proto/*)

proto: clean-proto get-deps $(addsuffix -protoc,$(PROTO_DIRS)) ##@Build Generate Go files from protobuf definitions

%-protoc:
	cd $(@:-protoc=) && protoc --proto_path=$(PWD):$(PWD)/vendor:/home/devbox/gopath/src/:. --gogo_out=plugins=grpc:. *.proto

clean-proto: $(addsuffix -clean,$(PROTO_DIRS)) ##@Clean Remove Go files generated from protobuf definitions
	rm -rf links
	rm -f proto

%-clean:
	cd $(@:-clean=) && rm -f *.pb.go && rm -f *pb_test.go

clean: clean-proto  ##@Clean Clean common libraries and generated code
	rm -rf $(ARTIFACTS_DIR)

test_dirs:=$(shell find pkg -name "*_test.go" -print | xargs -I{} dirname {} | uniq)

test: proto  ##@Test Run unit tests
	mkdir -p $(ARTIFACTS_DIR)/coverage || true
	echo "mode: count" > $(ARTIFACTS_DIR)/coverage/$(REPO_NAME)_coverage.out
	for pkg in $(test_dirs) ; do \
  		echo "\nTest directory: "$$pkg; \
  		mkdir -p $(ARTIFACTS_DIR)/coverage/$$pkg && \
  		sudo -E CGO_CFLAGS="$(ALCIDE_CGO_CFLAGS)" CGO_LDFLAGS="$(ALCIDE_CGO_LDFLAGS)" \
		$(GOROOT)/bin/go test -v -timeout 300s -covermode=count -coverprofile=$(ARTIFACTS_DIR)/coverage/$$pkg/cover.out $(REPO)/$$pkg || exit 1; \
		grep -h -v "^mode:" $(ARTIFACTS_DIR)/coverage/$$pkg/cover.out >> $(ARTIFACTS_DIR)/coverage/$(REPO_NAME)_coverage.out; \
	done
	go tool cover -html $(ARTIFACTS_DIR)/coverage/$(REPO_NAME)_coverage.out -o $(ARTIFACTS_DIR)/coverage/$(REPO_NAME)_coverage.html
	echo "Coverage created in: $(ARTIFACTS_DIR)/coverage/generic_agent_coverage.html"


get-deps: go.mod go.sum ##@Misc Get required Go libraries and tools
	go mod vendor

clean-get-deps: ##@Clean go.mod file and go.sum from unused Go libraries and tools
	go mod tidy
	$(MAKE) get-deps

go-format: ##@Misc Format go code and import packages
	gofmt -w -l ./pkg
	goimports -w -local $(REPO) ./pkg

go-lint: ##@Test Self code-review. Verify standards and discover potential bugs in your code
ifeq (,$(wildcard /home/devbox/gopath/bin/golangci-lint))
	go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.41.1
	go mod vendor
endif
	golangci-lint run pkg/...