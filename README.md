# Wistia S3

[![Go Report Card](https://goreportcard.com/badge/github.com/MMHK/wistia-s3)](https://goreportcard.com/report/github.com/MMHK/wistia-s3)
[![Docker Pulls](https://img.shields.io/docker/pulls/mmhk/wistia-s3)](https://hub.docker.com/r/mmhk/wistia-s3)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub issues](https://img.shields.io/github/issues/mmhk/wistia-s3)](https://github.com/mmhk/wistia-s3/issues)


将 Wistia Video 迁移至 Amazon S3。

## 项目目标

1. 实现一个功能与 Wistia 视频播放器相似的[视频播放器](https://github.com/MMHK/wistia-s3-player)。
2. 将 Wistia 上的视频上传至 S3。

## 编译 Go 项目

如果您希望从源代码编译此项目，请按照以下步骤操作：

1. 确保您已安装 Go 1.19 或更高版本。
2. 克隆此仓库：

    ```sh
    git clone https://github.com/MMHK/wistia-s3.git
    cd wistia-s3
    ```

3. 编译项目：

    ```sh
    go build -o wistia-s3
    ```

4. 运行编译后的可执行文件：

    ```sh
    ./wistia-s3
    ```

## 环境变量说明

在运行项目时，您需要设置以下环境变量：

- `S3_KEY`：您的 AWS S3 访问密钥。
- `S3_SECRET`：您的 AWS S3 秘密密钥。
- `S3_REGION`：您的 AWS S3 区域，例如 `ap-southeast-1`。
- `S3_BUCKET`：您的 AWS S3 存储桶名称。
- `S3_PREFIX`：存储在 S3 中的文件前缀，例如 `wistia-backup`。
- `S3_CLOUDFRONT_DOMAIN`：您的 CloudFront 域名。
- `WISTIA_API_KEY`：您的 Wistia API 密钥。
- `WISTIA_WORKER_LIMIT`：并发处理 Wistia 视频的工作线程数量。
- `TEMPLATE_DIR_PATH`：模板文件的目录路径。
- `LOG_LEVEL`：日志级别，例如 `INFO`、`DEBUG` 等。
- `TZ`：时区，例如 `Asia/Hong_Kong`。
- `LISTEN`：应用监听的地址和端口，例如 `0.0.0.0:3031`。
- `DB_FILE_PATH`：数据库文件的路径。
- `WEBROOT`：Web 根目录路径。

## 使用 Docker Compose

我们提供了一个 `docker-compose.yml` 文件来简化项目的运行。您可以按照以下步骤使用 Docker Compose 来启动项目：

1. 确保您已安装 Docker 和 Docker Compose。
2. 在项目根目录下创建 `docker-compose.yml` 文件，并添加以下内容：

    ```yaml
    version: '3.8'

    services:
      app:
        image: mmhk/wistia-s3:latest
        container_name: wistia-s3
        restart: always
        ports:
          - "3031:3031"
        environment:
          - S3_KEY=$S3_KEY
          - S3_SECRET=$S3_SECRET
          - S3_REGION=ap-southeast-1
          - S3_BUCKET=s3.test.mixmedia.com
          - S3_PREFIX=wistia-backup
          - S3_CLOUDFRONT_DOMAIN=$S3_CLOUDFRONT_DOMAIN
          - WISTIA_API_KEY=$WISTIA_API_KEY
          - WISTIA_WORKER_LIMIT=3
          - TEMPLATE_DIR_PATH=/app/web/dist
          - LOG_LEVEL=INFO
          - TZ=Asia/Hong_Kong
          - LISTEN=0.0.0.0:3031
          - DB_FILE_PATH=/app/wista-s3.db
          - WEBROOT=/app/webroot
    ```

3. 在项目根目录下运行以下命令来启动服务：

    ```sh
    docker-compose up
    ```

4. 服务启动后，您可以在浏览器中访问 `http://localhost:8080` 来查看运行中的应用。


## 获取 Wistia API 密钥

要获取您的 Wistia API 密钥，请按照以下步骤操作：

1. 登录到您的 Wistia 账户。
2. 在右上角，点击您的个人头像，然后选择 **[Account](https://account.wistia.com/account)** 。
3. 在账户设置页面，选择 **[API](https://account.wistia.com/account/api)** 选项卡。
4. 在 API 选项卡中，您可以看到现有的 API 密钥。如果没有，请点击 **Create New Token** 来生成一个新的 API 密钥。
5. 给您的 API 密钥命名，并选择所需的权限，然后点击 **Generate Token**。
6. 生成的 API 密钥将显示在页面上。请将其复制并安全保存。

请注意，API 密钥是敏感信息，不要在公共场合分享或暴露您的 API 密钥。

## 许可

本项目基于 Apache 2.0 许可进行分发。详情请参阅 [LICENSE](LICENSE) 文件。

---

感谢您对本项目的关注和支持！