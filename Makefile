.PHONY: proto proto-clean proto-check proto-deps install-tools run-dev run-scheduler run-executor run-web

# 获取 GOPATH/bin 路径
GOPATH_BIN := $(shell go env GOPATH)/bin
# 确保 GOPATH/bin 在 PATH 中
export PATH := $(GOPATH_BIN):$(PATH)

# 安装 protobuf 相关工具
install-tools:
	@echo "Installing protobuf tools..."
	@which protoc > /dev/null || (echo "❌ protoc not found, please install protobuf first (brew install protobuf on macOS)"; exit 1)
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32.0
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	@echo "✅ Protobuf tools installed successfully!"

# 检查 protobuf 相关依赖
proto-deps:
	@echo "Checking protobuf dependencies..."
	@which protoc > /dev/null || (echo "❌ protoc not found"; exit 1)
	@which protoc-gen-go > /dev/null || (echo "❌ protoc-gen-go not found, run 'make install-tools'"; exit 1)
	@which protoc-gen-go-grpc > /dev/null || (echo "❌ protoc-gen-go-grpc not found, run 'make install-tools'"; exit 1)
	@echo "✅ All protobuf dependencies found!"

# 清理旧的 proto 生成文件
proto-clean:
	@echo "Cleaning old proto generated files..."
	@rm -f proto/*.pb.go proto/*_grpc.pb.go
	@echo "✅ Old proto files cleaned!"

# 验证 proto 文件语法
proto-check: proto-deps
	@echo "Checking proto file syntax..."
	@protoc --proto_path=proto --proto_path=. proto/executor.proto
	@echo "✅ Proto file syntax check completed!"

# 生成 protobuf 代码
proto: proto-deps
	@echo "Generating protobuf code..."
	@protoc --proto_path=proto \
		--go_out=proto \
		--go_opt=paths=source_relative \
		--go-grpc_out=proto \
		--go-grpc_opt=paths=source_relative \
		proto/executor.proto
	@echo "✅ Protobuf code generated successfully!"
	@echo "   - proto/executor.pb.go"
	@echo "   - proto/executor_grpc.pb.go"

tidy:
	go mod tidy

setup-config:
	@echo "Setting up configuration files..."
	@mkdir -p logs
	@if [ ! -f scheduler/config.yaml ]; then \
		cp scheduler/config.yaml.example scheduler/config.yaml; \
		echo "Created scheduler/config.yaml"; \
	fi
	@if [ ! -f executor/config.yaml ]; then \
		cp executor/config.yaml.example executor/config.yaml; \
		echo "Created executor/config.yaml"; \
	fi

run-dev: setup-config
	@echo "=== Starting BDopsFlow Development Mode ==="
	@echo ""
	@echo "Building Scheduler..."
	@cd scheduler && mkdir -p bin && go build -o bin/scheduler ./cmd/main.go
	@echo "Building Executor..."
	@cd executor && mkdir -p bin && go build -o bin/executor ./cmd/main.go
	@echo ""
	@echo "Installing Web Dependencies..."
	@cd web && npm install
	@echo ""
	@echo "Starting Scheduler in background..."
	@cd scheduler && nohup ./bin/scheduler > ../logs/scheduler.log 2>&1 &
	@echo "Starting Executor in background..."
	@cd executor && nohup ./bin/executor > ../logs/executor.log 2>&1 &
	@echo "Starting Web Frontend..."
	@cd web && npx vite --host > ../logs/web.log 2>&1 &
	@sleep 2
	@echo ""
	@echo "=== BDopsFlow Development Mode Started ==="
	@echo ""
	@echo "Services:"
	@echo "  - Scheduler HTTP API: http://localhost:8080"
	@echo "  - Scheduler gRPC: localhost:50051"
	@echo "  - Frontend: http://localhost:3000"
	@echo ""
	@echo "Configuration:"
	@echo "  - Scheduler: scheduler/config.yaml"
	@echo "  - Executor: executor/config.yaml"
	@echo ""
	@echo "Logs:"
	@echo "  - Scheduler: logs/scheduler.log"
	@echo "  - Executor: logs/executor.log"
	@echo "  - Web: logs/web.log"
	@echo ""
	@echo "To stop all services:"
	@echo "  make stop-dev"
	@echo ""

stop-dev:
	@echo "Stopping BDopsFlow services..."
	@pkill -f 'scheduler' || true
	@pkill -f 'executor' || true
	@pkill -f 'vite' || true
	@echo "All services stopped."

run-scheduler: setup-config
	@echo "Starting Scheduler..."
	@cd scheduler && go run ./cmd/main.go

run-executor: setup-config
	@echo "Starting Executor..."
	@cd executor && go run ./cmd/main.go

run-web:
	@cd web && npm run dev
