.PHONY: proto build-server build-client build run-server clean tidy

PROTO_DIR := proto
BIN_DIR := bin
MODULE := ecs-plugin-dev

proto:
	mkdir -p $(PROTO_DIR)
	protoc \
		--go_out=. \
		--go-grpc_out=. \
		--go_opt=module=$(MODULE) \
		--go-grpc_opt=module=$(MODULE) \
		$(PROTO_DIR)/deployment.proto

build-server:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/grpc-server cmd/grpc-server/main.go

build-client:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/grpc-client cmd/grpc-client/main.go

build: proto build-server build-client

run-server:
	./$(BIN_DIR)/grpc-server

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)
