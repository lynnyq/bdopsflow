#!/bin/bash
set -e

echo "=== Starting BDopsFlow Development Environment ==="

# 1. 启动 Redis
echo "Starting Redis..."
docker start bdopsflow-redis 2>/dev/null || \
docker run -d --name bdopsflow-redis -p 6379:6379 redis:7-alpine

# 2. 启动 rqlite
echo "Starting rqlite..."
docker start bdopsflow-rqlite 2>/dev/null || \
docker run -d \
  --name bdopsflow-rqlite \
  -p 4001:4001 \
  -v bdopsflow-rqlite-data:/rqlite/file \
  rqlite/rqlite:latest

# 3. 等待服务启动
sleep 3

# 4. 初始化数据库（如果需要）
echo "Initializing database..."
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @deploy/schema.sql 2>/dev/null || true

# 5. 编译调度中心
echo "Building scheduler..."
cd scheduler
go build -o bin/scheduler ./cmd/main.go

# 6. 启动调度中心
echo "Starting scheduler..."
export HTTP_PORT=8080
export GRPC_PORT=50051
export RQLITE_DSN=http://localhost:4001
export REDIS_ADDR=localhost:6379
./bin/scheduler &
SCHEDULER_PID=$!

echo ""
echo "=== BDopsFlow Development Environment Started ==="
echo ""
echo "Services:"
echo "  - Scheduler HTTP API: http://localhost:8080"
echo "  - Scheduler gRPC: localhost:50051"
echo "  - rqlite: http://localhost:4001"
echo "  - Redis: localhost:6379"
echo "  - Frontend: http://localhost:5173 (run 'cd web && npm run dev' in another terminal)"
echo ""
echo "To stop all services:"
echo "  1. Kill scheduler: kill $SCHEDULER_PID"
echo "  2. Stop containers: docker stop bdopsflow-redis bdopsflow-rqlite"
echo ""
echo "Scheduler PID: $SCHEDULER_PID"
echo ""
echo "Press Ctrl+C to stop the scheduler..."
echo ""

# 等待信号
wait $SCHEDULER_PID
