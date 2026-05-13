.PHONY: proto run-dev run-scheduler run-executor run-web

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/executor.proto

tidy:
	go mod tidy

run-dev:
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
	@cd scheduler && \
	HTTP_PORT=8080 GRPC_PORT=50051 RQLITE_DSN=http://localhost:4001 REDIS_ADDR=localhost:6379 \
	nohup ./bin/scheduler > ../logs/scheduler.log 2>&1 &
	@echo "Starting Executor in background..."
	@cd executor && \
	GRPC_SERVER=localhost:50051 REDIS_ADDR=localhost:6379 \
	nohup ./bin/executor > ../logs/executor.log 2>&1 &
	@echo "Starting Web Frontend..."
	@cd web && npx vite > ../logs/web.log 2>&1 &
	@echo ""
	@echo "=== BDopsFlow Development Mode Started ==="
	@echo ""
	@echo "Services:"
	@echo "  - Scheduler HTTP API: http://localhost:8080"
	@echo "  - Scheduler gRPC: localhost:50051"
	@echo "  - Frontend: http://localhost:5173"
	@echo ""
	@echo "Logs:"
	@echo "  - Scheduler: logs/scheduler.log"
	@echo "  - Executor: logs/executor.log"
	@echo "  - Web: logs/web.log"
	@echo ""
	@echo "To stop all services:"
	@echo "  pkill -f 'bdopsflow-scheduler' && pkill -f 'bdopsflow-executor' && pkill -f 'vite'"
	@echo ""
