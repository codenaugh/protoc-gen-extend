PLUGIN_BIN := $(shell go env GOPATH)/bin/protoc-gen-extend

.PHONY: build install test proto example clean

build:
	go build -o protoc-gen-extend .

install:
	go install .

test:
	go test -v ./...

proto: install
	protoc \
		--proto_path=example \
		--go_out=example/userpb \
		--go_opt=paths=source_relative \
		--plugin=protoc-gen-extend=$(PLUGIN_BIN) \
		--extend_out=sidecar_root=example:example/userpb \
		--extend_opt=paths=source_relative \
		example/user.proto

example: proto
	@echo "Generated files:"
	@ls -la example/userpb/
	@echo ""
	@echo "=== user_ext.pb.go ==="
	@cat example/userpb/user_ext.pb.go

clean:
	rm -f protoc-gen-extend
	rm -rf example/userpb/*.pb.go
