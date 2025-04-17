# Docker Bundle 示例

这是一个完整的示例，展示如何使用 dockerbundle 工具打包和还原一个典型的 Web 应用。

## 应用架构

这个示例应用包含三个主要组件：

1. **Web 服务器 (Nginx)**
   - 提供静态文件服务
   - 反向代理 API 请求到 Node.js 应用
   - 详细的访问日志记录

2. **应用服务器 (Node.js)**
   - 提供 REST API
   - 连接数据库
   - 处理业务逻辑
   - 结构化日志输出

3. **数据库 (PostgreSQL)**
   - 存储应用数据
   - 使用 Docker volume 持久化数据
   - 数据库日志记录

## 目录结构

```
example/
├── docker-compose.yml      # Docker Compose 配置
├── .env                    # 环境变量
├── nginx/                  # Nginx 配置
│   ├── conf.d/            # Nginx 配置文件
│   │   └── default.conf
│   └── html/              # 静态文件
│       └── index.html
├── app/                    # Node.js 应用
│   ├── package.json
│   └── server.js
├── logs/                  # 日志目录
│   ├── nginx/            # Nginx 日志
│   ├── app/              # 应用日志
│   └── postgres/         # 数据库日志
└── secrets/               # 敏感信息
    └── db_password        # 数据库密码
```

## 日志系统

### 1. Nginx 日志
- **访问日志**: `/logs/nginx/access.log`
  - 详细的 JSON 格式日志
  - 包含请求时间、客户端信息、响应状态等
- **API 访问日志**: `/logs/nginx/access.api.log`
  - 专门记录 API 请求
- **静态文件访问日志**: `/logs/nginx/access.static.log`
  - 记录静态资源访问
- **错误日志**: `/logs/nginx/error.log`

### 2. 应用日志
Node.js 应用使用结构化日志记录：
- 所有日志以 JSON 格式输出
- 包含时间戳、日志级别、消息和元数据
- 记录以下事件：
  - 服务器启动和关闭
  - 数据库连接状态
  - API 请求和响应
  - 错误和异常

### 3. 数据库日志
PostgreSQL 日志保存在 `/logs/postgres` 目录：
- 查询日志
- 错误日志
- 连接日志

### 4. Docker 日志配置
所有服务都配置了日志轮转：
- 最大文件大小：10MB
- 保留 3 个历史文件
- 使用 JSON 日志驱动

## 使用方法

1. **打包应用**

```bash
# 在 example 目录下运行
dockerbundle -c docker-compose.yml -o bundle
```

这将创建一个包含所有必要资源的 bundle 目录：
- Docker 镜像 (nginx, node, postgres)
- 配置文件
- 卷数据
- 环境变量和密钥

2. **还原应用**

```bash
# 解压 bundle（如果已压缩）
tar xzf bundle.tar.gz

# 运行还原脚本
./scripts/restore.sh ./bundle

# 启动服务
docker-compose up -d
```

3. **访问应用**

- Web 界面：http://localhost:8080
- API 健康检查：http://localhost:8080/api/health
- API 数据端点：http://localhost:8080/api/items

## 日志查看

1. **实时查看日志**
```bash
# Nginx 访问日志
docker-compose logs -f web

# 应用日志
docker-compose logs -f app

# 数据库日志
docker-compose logs -f db
```

2. **查看特定日志文件**
```bash
# Nginx API 日志
tail -f logs/nginx/access.api.log

# 应用日志
tail -f logs/app/app.log

# 数据库日志
tail -f logs/postgres/postgresql.log
```

3. **日志分析**
```bash
# 统计 API 请求状态码
cat logs/nginx/access.api.log | jq -r .status | sort | uniq -c

# 查看慢请求
cat logs/nginx/access.log | jq 'select(.request_time > 1)'

# 查看错误请求
cat logs/nginx/access.log | jq 'select(.status >= "400")'
```

## 注意事项

1. **安全性**
   - 示例中的密码仅用于演示
   - 在生产环境中应使用强密码
   - 注意保护敏感文件

2. **配置**
   - 可以根据需要修改 Nginx 配置
   - 环境变量可以根据部署环境调整
   - 数据库连接参数可以自定义

3. **数据持久化**
   - PostgreSQL 数据存储在命名卷中
   - 可以通过 Docker volume 备份数据

## 自定义

1. **添加更多服务**
   - 在 docker-compose.yml 中添加新服务
   - 添加相应的配置文件
   - 更新 Nginx 配置进行代理

2. **修改配置**
   - 调整端口映射
   - 修改环境变量
   - 更新数据库设置

3. **扩展功能**
   - 添加更多 API 端点
   - 集成其他数据库
   - 添加监控和日志 