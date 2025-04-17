#!/bin/bash

# 启用错误处理
set -e

# 检查参数
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <bundle_directory>"
    exit 1
fi

BUNDLE_DIR="$1"
cd "$BUNDLE_DIR"

# 检查必要文件和目录
if [ ! -f "manifest.json" ]; then
    echo "Error: manifest.json not found in bundle directory"
    exit 1
fi

# 创建目标目录
TARGET_DIR="../target"
mkdir -p "$TARGET_DIR"

echo "Starting restoration process..."
echo "Target directory: $TARGET_DIR"

# 1. 加载 Docker 镜像
if [ -d "images" ]; then
    shopt -s nullglob
    images=(images/*.tar)
    if [ ${#images[@]} -gt 0 ]; then
        echo "Loading Docker images..."
        for image in "${images[@]}"; do
            echo "Loading image: $image"
            docker load < "$image"
        done
        echo "All Docker images loaded successfully"
    else
        echo "No Docker images found to restore"
    fi
    shopt -u nullglob
fi

# 2. 还原 secrets
if [ -d "secrets" ]; then
    shopt -s nullglob
    secrets=(secrets/*)
    if [ ${#secrets[@]} -gt 0 ]; then
        echo "Restoring secrets..."
        mkdir -p "$TARGET_DIR/secrets"
        cp -r secrets/. "$TARGET_DIR/secrets/" 2>/dev/null || true
        chmod 700 "$TARGET_DIR/secrets"
        find "$TARGET_DIR/secrets" -type f -exec chmod 600 {} \;
        echo "Secrets restored successfully"
    else
        echo "No secrets found to restore"
    fi
    shopt -u nullglob
fi

# 3. 创建并还原卷数据
if [ -d "volumes" ]; then
    echo "Processing volumes..."
    
    # 还原命名卷
    if [ -d "volumes/_named" ]; then
        shopt -s nullglob
        named_volumes=(volumes/_named/*)
        if [ ${#named_volumes[@]} -gt 0 ]; then
            echo "Restoring named volumes..."
            for volume_dir in "${named_volumes[@]}"; do
                if [ -d "$volume_dir" ]; then
                    volume_name=$(basename "$volume_dir")
                    echo "Creating named volume: $volume_name"
                    docker volume create "$volume_name" || true
                    if [ -n "$(ls -A "$volume_dir" 2>/dev/null)" ]; then
                        echo "Restoring data for volume: $volume_name"
                        docker run --rm \
                            -v "$volume_name":/target \
                            -v "$(pwd)/$volume_dir":/source \
                            alpine sh -c "cp -r /source/. /target/"
                    fi
                fi
            done
            echo "Named volumes restored successfully"
        else
            echo "No named volumes found to restore"
        fi
        shopt -u nullglob
    fi

    # 还原 bind mount 卷
    if [ -d "volumes/bind" ]; then
        shopt -s nullglob
        services=(volumes/bind/*)
        if [ ${#services[@]} -gt 0 ]; then
            echo "Restoring bind mount volumes..."
            for service_dir in "${services[@]}"; do
                if [ -d "$service_dir" ]; then
                    service_name=$(basename "$service_dir")
                    echo "Restoring bind mounts for service: $service_name"
                    if [ -n "$(ls -A "$service_dir" 2>/dev/null)" ]; then
                        if [ "$service_name" = "redis" ]; then
                            # Special handling for Redis configuration
                            mkdir -p "$TARGET_DIR/redis/conf"
                            if [ -f "$service_dir/redis.conf/redis.conf" ]; then
                                cp "$service_dir/redis.conf/redis.conf" "$TARGET_DIR/redis/conf/redis.conf"
                            fi
                        else
                            mkdir -p "$TARGET_DIR/$service_name"
                            cp -r "$service_dir/." "$TARGET_DIR/$service_name/" 2>/dev/null || true
                        fi
                    fi
                fi
            done
            echo "Bind mount volumes restored successfully"
        else
            echo "No bind mount volumes found to restore"
        fi
        shopt -u nullglob
    fi
fi

# 4. 还原配置文件
if [ -d "configs" ]; then
    shopt -s nullglob dotglob
    configs=(configs/*)
    echo "In config:" "${configs[@]}"
    shopt -u nullglob dotglob
    if [ ${#configs[@]} -gt 0 ]; then
        echo "Restoring config files..."
        mkdir -p "$TARGET_DIR/configs"
        cp -r configs/. "$TARGET_DIR/configs/" 2>/dev/null || true
        echo "Config files restored successfully"
    else
        echo "No config files found to restore"
    fi
    shopt -u nullglob
fi

# 5. 复制 docker-compose 文件和环境文件
if [ -d "compose" ]; then
    shopt -s nullglob dotglob  # 启用隐藏文件
    compose_files=(compose/*)
    if [ ${#compose_files[@]} -gt 0 ]; then
        echo "Copying docker-compose files..."
        cp -r compose/. "$TARGET_DIR/" 2>/dev/null || true
        echo "Docker compose files copied successfully"
    else
        echo "No docker-compose files found"
    fi
    shopt -u nullglob dotglob
fi

# 6. 显示使用说明
echo
echo "Bundle restored successfully!"
echo "Summary of restored resources:"
[ -d "images" ] && echo "- Docker images: $(ls images/*.tar 2>/dev/null | wc -l) images"
[ -d "secrets" ] && echo "- Secrets: $(ls secrets/ 2>/dev/null | wc -l) files"
[ -d "volumes/_named" ] && echo "- Named volumes: $(ls volumes/_named/ 2>/dev/null | wc -l) volumes"
[ -d "volumes/bind" ] && echo "- Bind mounts: $(ls volumes/bind/ 2>/dev/null | wc -l) services"
[ -d "configs" ] && echo "- Config files: $(ls configs/ 2>/dev/null | wc -l) files"
[ -d "compose" ] && echo "- Compose files: $(ls -A compose/ 2>/dev/null | wc -l) files"

echo
echo "Next steps:"
echo "1. Review the restored files in: $TARGET_DIR"
echo "2. Run: docker-compose -f $TARGET_DIR/docker-compose.yml up -d" 