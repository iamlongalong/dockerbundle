#!/bin/bash
set -e

# 清理旧的测试数据
cleanup() {
    echo "Cleaning up..."
    rm -rf bundle
    docker-compose -f example/docker-compose.yml down -v 2>/dev/null || true
}

# 在脚本退出时清理
trap cleanup EXIT

# 进入示例目录
cd example

# 启动服务
echo "Starting services..."
docker-compose up -d
sleep 5  # 等待服务启动

# 写入一些测试数据到 Redis
echo "Writing test data..."
docker-compose exec -T redis redis-cli set test_key "test_value"

docker-compose down

# 使用 dockerbundle 打包
echo "Creating bundle..."
../dockerbundle -a -v -o ../bundle

# 创建必要的目录和文件
echo "Creating required directories and files..."
cd ../bundle

tree

chmod +x scripts/restore.sh

# 验证打包结果
echo "Verifying bundle structure..."
required_dirs=("compose" "images" "volumes" "configs" "scripts" "secrets")
for dir in "${required_dirs[@]}"; do
    if [ ! -d "$dir" ]; then
        echo "Error: Required directory $dir not found in bundle"
        exit 1
    fi
done

# 验证必要文件
if [ ! -f "manifest.json" ]; then
    echo "Error: manifest.json not found"
    exit 1
fi

if [ ! -f "scripts/restore.sh" ]; then
    echo "Error: restore.sh not found"
    exit 1
fi

# 验证 secrets 是否正确打包
if [ ! -f "secrets/redis_password" ]; then
    echo "Error: redis_password secret not found in bundle"
    exit 1
fi

# 停止并清理现有服务
echo "Stopping services..."
cd ../example
docker-compose down -v

# 测试还原功能
echo "Testing restore functionality..."
cd ../bundle
chmod +x scripts/restore.sh
./scripts/restore.sh .

# 启动还原后的服务
cd ../example
docker-compose up -d
sleep 5  # 等待服务启动

# 验证数据
echo "Verifying restored data..."
REDIS_PASSWORD=$(cat secrets/redis_password)
test_value=$(docker-compose exec -T redis redis-cli -a "$REDIS_PASSWORD" get test_key)
if [ "$test_value" != "test_value" ]; then
    echo "Error: Data verification failed"
    echo "Expected: test_value"
    echo "Got: $test_value"
    exit 1
fi

# 验证 secrets 是否正确还原
if [ ! -f "secrets/redis_password" ]; then
    echo "Error: redis_password secret not restored"
    exit 1
fi

restored_password=$(cat secrets/redis_password)
if [ "$restored_password" != "mysecretpassword123" ]; then
    echo "Error: redis_password content mismatch"
    exit 1
fi

echo "All tests passed successfully!" 