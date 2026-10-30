# 文档网站维护

文档站使用 VitePress，将仓库中的 Markdown 直接构建为静态网站；源码和网站共用一份内容。公开地址为 [SSHGuard 文档](https://xiuzhenzhi.github.io/SSH/)，由 GitHub Pages 托管，无需占用主控服务器。

## 自动发布

修改 `docs/`、根目录 Markdown、版本文件或文档站配置，推送到 `main` 后，GitHub Actions 的 **Documentation** 工作流会检查链接、构建并部署网站。PR 只构建验证，不发布。文档站独立于主控发行包更新，普通文档改动无需增加程序版本。

首次使用需在仓库 Settings → Pages 将 Source 设为 GitHub Actions。工作流使用 `pages:write` 与 `id-token:write` 部署；代码检查本身只需读取权限。不需要在仓库中保存长期部署密钥。

## 本地预览

在项目根目录执行（PowerShell 使用 `npm.cmd`）：

```sh
npm ci
npm run docs:dev
```

生产构建与预览：

```sh
npm run docs:build
npm run docs:preview
```

构建产物和缓存仅放在被 Git 忽略的 `.local/docs-site/`，不会进入原生发行包。`web/package.json` 只构建主控控制台，根目录 `package.json` 只构建文档，两者独立。

预览服务使用构建时的快照；重新构建后重启 `docs:preview` 再检查。日常写作使用 `docs:dev` 自动刷新。稳定版 VitePress 的 Vite 依赖通过根目录 overrides 固定到兼容的安全补丁版本，升级时保留构建和依赖审计检查。

## 编辑规则

- 正文直接修改现有 Markdown，侧栏和导航在 `docs/.vitepress/config.mts`，首页在 `docs/index.md`。
- 中文本地搜索无需第三方搜索服务；代码块提供复制按钮，支持浅深色与手机导航。
- 版本导航从根目录 `VERSION` 读取，最后更新时间来自 Git。改动安装命令时同步 README、快速开始与运维说明。
- GitHub Pages 子路径是 `/SSH/`，配置中的 `base` 和站点图标路径须一致；自定义域名时同步修改并重新构建。
- 文档部署不更新主控，也不会覆盖旧解压目录；主控程序和随包文档仍通过正式 Release 更新。

参考：[VitePress](https://vitepress.dev/)、[GitHub Pages 部署说明](https://vitepress.dev/guide/deploy#github-pages)。
