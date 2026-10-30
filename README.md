# SSHGuard

集中管理 Windows / Linux 服务器的 SSH 登录记录与安全告警。

## 功能

- SSH 审计：查看登录时间、用户、来源 IP、端口和认证结果。
- 安全告警：异常登录、连续认证失败、服务器离线与采集异常通知。
- 服务器管理：分组、IP 白名单、Agent 安装与远程更新。
- 登录防护：人机验证、登录错误封禁、用户权限与操作审计。

支持 Windows、Ubuntu、Debian、Alpine；数据库可选 SQLite、PostgreSQL 或 MySQL。

![安全总览](docs/overview-preview.png)

## 安装

主控使用 Ubuntu / Debian，默认通过 IP + 端口访问，使用 SQLite，无需域名或 Docker。

```sh
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
sh scripts/quick-start.sh
```

打开安装器给出的地址，创建管理员。在「服务器管理」添加服务器，复制对应系统的安装命令，在目标机器执行即可。

更新主控到最新正式版（自动下载、校验并保留配置和数据）：

```sh
sh scripts/update.sh
```

安装后无需保留源码目录，也可运行 `sh /opt/sshguard/update.sh`。管理菜单中「1」安装主控，「9」下载最新正式版并更新；已安装时回车默认选择「9」。详细步骤和离线更新见 [安装与管理](docs/QUICKSTART.md)。

## 文档

[安装与管理](docs/QUICKSTART.md) · [Compose 安装](docs/INSTALL-COMPOSE.md) · [Agent](docs/AGENTS.md) · [反代与 HTTPS](docs/REVERSE-PROXY.md) · [告警配置](docs/ALERTING.md) · [登录防护](docs/LOGIN-SECURITY.md)

[更新日志](CHANGELOG.md) · [下载版本](https://github.com/xiuzhenzhi/SSH/releases) · [参与贡献](CONTRIBUTING.md) · [安全反馈](SECURITY.md)
