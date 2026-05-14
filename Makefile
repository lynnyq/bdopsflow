.PHONY: proto run-dev run-scheduler run-executor run-web

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/executor.proto

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
