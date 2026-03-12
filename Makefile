PLUGIN_BIN := $(shell go env GOPATH)/bin/protoc-gen-methods

.PHONY: build install test proto example clean

build:
	go build -o protoc-gen-methods .

install:
	go install .

test:
	go test -v ./...

proto: install
	protoc \
		--proto_path=example \
		--go_out=example/userpb \
		--go_opt=paths=source_relative \
		--plugin=protoc-gen-methods=$(PLUGIN_BIN) \
		--methods_out=sidecar_root=example:example/userpb \
		--methods_opt=paths=source_relative \
		example/user.proto

example: proto
	@echo "Generated files:"
	@ls -la example/userpb/
	@echo ""
	@echo "=== user_methods.pb.go ==="
	@cat example/userpb/user_methods.pb.go

clean:
	rm -f protoc-gen-methods
	rm -rf example/userpb/*.pb.go
