import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"SSHGuard 文档","description":"","frontmatter":{"layout":"home","title":"SSHGuard 文档","hero":{"name":"SSHGuard","text":"让每一次 SSH 登录<br>都有迹可循","tagline":"集中查看 Windows 与 Linux 服务器的登录记录，管理安全告警，让服务器状态清楚可见。","image":{"src":"/sshguard.svg","alt":"SSHGuard 盾牌与终端标志"},"actions":[{"theme":"brand","text":"开始安装","link":"/docs/QUICKSTART"},{"theme":"alt","text":"更新已有主控","link":"/docs/QUICKSTART#拉取最新更新脚本"},{"theme":"alt","text":"查看 GitHub","link":"https://github.com/xiuzhenzhi/SSH"}]},"features":[{"title":"一键安装与更新","details":"无需域名、Docker 或额外数据库，即可通过 IP 和端口开始使用；已有配置和数据在更新时保留。","link":"/docs/QUICKSTART","linkText":"部署主控"},{"title":"跨平台 SSH 审计","details":"支持 Windows、Ubuntu、Debian 和 Alpine，集中查看登录时间、账号、来源地址和认证结果。","link":"/docs/AGENTS","linkText":"接入服务器"},{"title":"告警与通知","details":"关注异常登录、认证失败、服务器离线和采集异常，将通知送到你使用的渠道。","link":"/docs/ALERTING","linkText":"配置告警"},{"title":"公网 IP 与来源国家","details":"识别公网 IPv4 / IPv6，在服务器和事件中显示国家与地区，查看认证失败来源国家排行。","link":"/docs/GEOIP","linkText":"了解地理位置"},{"title":"数据由你掌控","details":"自托管部署，可选 SQLite、MySQL 或 PostgreSQL；配置、数据库和加密密钥都由你维护。","link":"/docs/OPERATIONS","linkText":"备份与运维"},{"title":"从使用到开发","details":"安装指南、故障排查、API、传输协议和发布流程集中在这里，按你的需要逐步查阅。","link":"/docs/README","linkText":"浏览全部文档"}]},"headers":[],"relativePath":"index.md","filePath":"docs/index.md","lastUpdated":null}');
const _sfc_main = { name: "index.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("docs/index.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const index = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  index as default
};
