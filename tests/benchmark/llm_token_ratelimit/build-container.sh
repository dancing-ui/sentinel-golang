#!/bin/zsh

echo "Building application..."
./build-code.sh

echo "Building and starting containers..."
docker-compose -f docker-compose.yml down
docker-compose -f docker-compose.yml up --build -d

echo "Waiting for Redis nodes to start..."
sleep 10

echo "Checking Redis nodes status..."
for port in 7001 7002 7003; do
    echo "Checking Redis on port $port..."
    timeout 5 redis-cli -h 127.0.0.1 -p $port ping || echo "Redis on port $port not ready"
done

echo "Initializing Redis cluster..."
# 使用容器内部执行集群初始化
docker exec redis-node-1 redis-cli --cluster create \
    172.20.0.11:6379 \
    172.20.0.12:6379 \
    172.20.0.13:6379 \
    --cluster-replicas 0 \
    --cluster-yes

echo "Checking cluster status..."
docker exec redis-node-1 redis-cli cluster nodes

echo "Cluster setup complete!"