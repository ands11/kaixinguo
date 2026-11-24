# 部署签名服务

开心果取播放地址时，请求要带上红果 App 的签名。平时用的是 [openhongguo](https://github.com/beilunyang/openhongguo) 内置的签名，不需要任何服务器。红果收紧签名校验以后，内置签名可能会突然失效，所有剧集都播不了，要等 openhongguo 和开心果发新版才能恢复，这在 2026 年 11 月出现过三次。

等新版期间，可以自己部署一个 [openhongguo-signer](https://github.com/beilunyang/openhongguo/blob/main/signer/README.md) 签名服务，在开心果里填上它的地址，播放就能先恢复。它直接运行红果 App 自带的签名库，出问题时一般只要更新镜像，不用等客户端发版。

macOS、iOS / Apple TV 和 Android 版的开心果都能用同一个签名服务。

## 开心果怎样用它

- **默认不用**。设置里不填地址，开心果只用内置签名，和以前完全一样。
- **只有播放请求会用到它**。推荐、搜索、剧集列表、弹幕等其他请求一律只走内置签名，搜不到剧之类的空结果不会发给签名服务。
- **内置签名优先**。取播放地址时先用内置签名；拿到空结果或签名被拒，再交给签名服务签一次。签名服务成功了，之后 30 分钟的播放先走签名服务，过后再试回内置签名。
- **签名服务挂了不影响原来的行为**。连不上签名服务时退回内置签名，界面上显示的仍然是内置签名的错误。
- 用签名服务时另外注册一台匿名设备，和内置签名用的设备互不影响。

经过签名服务的，是播放请求的地址和请求体，里面有匿名设备的 ID 和要播的分集 ID，没有任何账号信息（开心果本来就不登录）。

## 需要什么

- 一台 Linux 服务器，装好 Docker 和 Docker Compose 插件。镜像支持 amd64 和 arm64。签名服务实测占用约 100 MB 内存，CPU 几乎不占，1 核 1 GB 的小服务器就够。
- 一个指向这台服务器的域名，用来开 HTTPS（下面用 `signer.example.com` 代替）。只在自己的电脑上用的话可以不要，见[只在本机用](#只在本机用)。

服务器最好在国内或离国内近，签名服务要和开心果来回通信，延迟会直接加到起播时间上。

## 部署

### 1. 启动签名服务

在服务器上建一个目录，生成 token：

```sh
mkdir -p ~/openhongguo-signer && cd ~/openhongguo-signer
(umask 077; echo "SIGNER_TOKEN=$(openssl rand -hex 32)" > .env)
```

**一定要设 token**：能连上签名服务的人都能用它签任意请求。

写一个 `docker-compose.yml`：

```yaml
services:
  openhongguo-signer:
    image: beilunyang/openhongguo-signer:v0.4.1
    container_name: openhongguo-signer
    restart: unless-stopped
    environment:
      SIGNER_TOKEN: ${SIGNER_TOKEN:?set SIGNER_TOKEN in .env}
    ports:
      - "127.0.0.1:19830:19830"   # 只在本机监听，由下面的反向代理对外
    mem_limit: 640m
    logging:
      driver: json-file
      options: { max-size: "10m", max-file: "3" }
```

镜像版本和开心果所用的 openhongguo 版本保持一致即可，版本列表见 [Docker Hub](https://hub.docker.com/r/beilunyang/openhongguo-signer/tags)。启动：

```sh
docker compose up -d
curl -s http://127.0.0.1:19830/ready   # 10～20 秒后返回 "ready":true
```

端口只绑在 `127.0.0.1`，不要改成 `0.0.0.0` 直接对外：Docker 写的 iptables 规则会绕过 ufw，防火墙挡不住它。

### 2. 用 HTTPS 对外

用 Caddy 做反向代理，它会自动申请证书。在 Caddyfile 里加上：

```caddyfile
signer.example.com {
	@signer path /ready /feature-hash
	handle @signer {
		reverse_proxy 127.0.0.1:19830
	}
	respond 404
}
```

- 只放行签名服务的两个接口，其他路径一律 404。
- 不要加 `encode`：SDK 要求签名服务的响应不压缩。
- 服务器的防火墙要放行 80 和 443（80 用来申请证书）。

重新加载 Caddy（`systemctl reload caddy`），然后在自己的电脑上检查：

```sh
curl -s https://signer.example.com/ready
# {"service":"openhongguo-signer","ready":true}
curl -s -o /dev/null -w '%{http_code}\n' -X POST -d '{}' https://signer.example.com/feature-hash
# 401：没带 token 被拒，说明 token 生效了
```

用 nginx 等其他反向代理也行，要求一样：只转发这两个路径，不压缩响应。

### 3. 在开心果里填上

- **macOS**：菜单「开心果 → 设置…」（⌘,）。
- **iOS、Android**：「我的」页右上角的齿轮。
- **Apple TV**：「我的」页底部的「设置」。

「地址」填 `https://signer.example.com`，「Token」填 `.env` 里 `SIGNER_TOKEN=` 后面的值。保存时开心果会先检查一遍，能分辨连不上、还在启动、token 不对这几种情况。签名服务暂时不在线也可以点「仍然保存」。

手机上请用 `https://` 地址。iOS 只允许检查局域网里的 `http://` 地址，Android 完全不能检查 `http://` 地址，这时会提示检查失败，点「仍然保存」也能用，但最好还是开 HTTPS。

清空地址再保存，就不再使用签名服务。

## 只在本机用

不想开服务器的话，也可以在自己的 Mac 上跑（需要 Docker Desktop 或 OrbStack）：

```sh
docker run -d --name openhongguo-signer --restart unless-stopped \
  -p 127.0.0.1:19830:19830 beilunyang/openhongguo-signer:v0.4.1
```

在 macOS 版开心果里填 `http://127.0.0.1:19830`，Token 留空：没设 `SIGNER_TOKEN` 时签名服务不校验 token，端口又只绑在本机，别的机器连不到。手机也连不到这个地址，所以这种方式只适合 macOS 版。

## 升级

内置签名再失效时，先看 [openhongguo 的发布说明](https://github.com/beilunyang/openhongguo/releases)，签名服务有修复时会提到新的镜像版本。改 `docker-compose.yml` 里的版本号，再执行：

```sh
docker compose pull && docker compose up -d
```

开心果这边什么都不用改。

## 排查

- **开心果提示「连不上签名服务」**：在电脑上 `curl https://signer.example.com/ready` 看看。连不上的话，查域名解析、防火墙的 80/443、Caddy 日志（`journalctl -u caddy`）。
- **签名服务一直不 ready**：`docker logs openhongguo-signer`。正常启动会打印 `HTTP server listening on 0.0.0.0:19830 (token required)`。
- **填了签名服务还是播不了**：说明签名服务的签名也被拒了，通常要等 openhongguo 发新镜像。签名服务连续签名失败时会自己退出，由 Docker 重启。
