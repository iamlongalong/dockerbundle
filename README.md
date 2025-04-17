# Docker Bundle

Docker Bundle 是一个用于打包和还原 Docker Compose 应用的工具。它可以将应用所需的所有资源（包括镜像、配置文件、数据卷等）打包到一个目录中，方便在其他环境中还原和部署。

## 安装

你可以通过以下几种方式安装 Docker Bundle：

### 从 GitHub Releases 下载

访问 [GitHub Releases](https://github.com/iamlongalong/dockerbundle/releases) 页面，下载适合你系统的二进制文件。

例如，对于 Linux x86_64 系统：
```bash
# 下载最新版本
curl -L https://github.com/iamlongalong/dockerbundle/releases/latest/download/dockerbundle_Linux_x86_64.tar.gz -o dockerbundle.tar.gz

# 解压
tar xzf dockerbundle.tar.gz

# 移动到系统路径
sudo mv dockerbundle /usr/local/bin/
```

### 使用 Go Install

如果你已经安装了 Go 1.16 或更高版本，可以直接使用 go install 命令安装：

```bash
go install github.com/iamlongalong/dockerbundle/cmd/dockerbundle@latest
```

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/iamlongalong/dockerbundle.git
cd dockerbundle

# 构建
go build ./cmd/dockerbundle
```

## 环境要求

### 打包环境
- Docker Engine 20.10.0 或更高版本
- Docker Compose v2.0.0 或更高版本
- Go 1.16 或更高版本（仅在构建工具时需要）

### 还原环境
- Docker Engine 20.10.0 或更高版本
- Docker Compose v2.0.0 或更高版本
- bash shell
- 足够的磁盘空间（取决于应用大小）

## 使用方法

### 1. 打包应用

```bash
dockerbundle -v -o bundle
```

打包完成后，会在指定的输出目录（默认为 `bundle`）中生成以下内容：
- `compose/` - Docker Compose 配置文件
- `images/` - Docker 镜像文件
- `volumes/` - 数据卷内容
  - `_named/` - 命名卷数据
  - `bind/` - 按服务分类的 bind mount 数据
- `scripts/` - 还原脚本
- `manifest.json` - 资源清单

### 2. 还原应用

1. 如果打包文件是压缩格式，首先解压：
```bash
tar xzf bundle.tar.gz
cd bundle
```

2. 运行还原脚本：
```bash
chmod +x scripts/restore.sh
./scripts/restore.sh .
```

还原脚本会自动执行以下操作：
- 加载所有 Docker 镜像
- 创建并还原数据卷
- 还原配置文件
- 提供后续操作指导

3. 启动应用：
```bash
docker-compose -f compose/docker-compose.yml up -d
```

## 目录结构说明

```
bundle/
├── compose/
│   └── docker-compose.yml    # Docker Compose 配置文件
├── images/
│   ├── image1.tar           # Docker 镜像文件
│   └── image2.tar
├── volumes/
│   ├── _named/             # 命名卷数据
│   │   └── volume1/
│   └── bind/               # Bind Mount 数据（按服务分类）
│       ├── service1/
│       │   └── data/
│       └── service2/
│           └── config/
├── scripts/
│   └── restore.sh          # 还原脚本
└── manifest.json           # 资源清单
```

## TODO
- restore 的脚本中，对 configs 下的文件还原有些问题，比如 .env 位置就不对
- restore 的脚本中，对 named volumes 的还原有些问题，没有还原
