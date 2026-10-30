import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"安全策略","description":"","frontmatter":{},"headers":[],"relativePath":"SECURITY.md","filePath":"SECURITY.md","lastUpdated":1793353928000}');
const _sfc_main = { name: "SECURITY.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="安全策略" tabindex="-1">安全策略 <a class="header-anchor" href="#安全策略" aria-label="Permalink to &quot;安全策略&quot;">​</a></h1><p>发现鉴权绕过、凭证泄露、安装脚本风险或其他漏洞时，不要在公开 Issue / PR 中放利用细节、有效令牌或真实日志。仓库开启私密漏洞报告时，使用 Security → Report a vulnerability；否则通过已知维护者的私下渠道联系，先提供脱敏摘要。</p><p>报告应包含版本 / 提交、受影响组件、可复现条件、脱敏步骤和影响范围。不要未经授权测试其他人的平台或服务器。</p><h2 id="身份与权限" tabindex="-1">身份与权限 <a class="header-anchor" href="#身份与权限" aria-label="Permalink to &quot;身份与权限&quot;">​</a></h2><p>登录支持服务端人机验证；来源 IP 在 60 秒内错误 5 次封禁 24 小时。修改密码撤销该用户全部会话。Cookie、Origin、角色和 Agent 凭证限制分别在服务端验证。登录防护配置加密保存，见 <a href="./docs/LOGIN-SECURITY.html">配置说明</a>。</p><p>采集服务使用低权限账号。Linux 凭证由 root 持有、服务只读；Windows 以 LocalService 采集，程序和凭证只读。API 与 Agent 使用独立服务账号。平台更新只领取固定版本任务，由预置的 root / SYSTEM 更新器处理，没有通用远程 Shell 或远程卸载接口。</p><h2 id="传输与下载" tabindex="-1">传输与下载 <a class="header-anchor" href="#传输与下载" aria-label="Permalink to &quot;传输与下载&quot;">​</a></h2><p>主控可显式启用 IP + HTTP 直连；登录、令牌和上报在 HTTP 下未加密，应使用可信网络或配置 HTTPS。Agent 的 HTTP 接入需显式授权，HTTPS 不自动降级或跳过证书验证。反代仅信任明确配置的来源，见 <a href="./docs/REVERSE-PROXY.html">反代说明</a>。</p><p>Agent 安装与更新直接下载固定 GitHub Release，限制 HTTPS 重定向、文件大小和超时，校验 SHA-256 与预期版本后才替换服务。SHA-256 是完整性校验，不是独立代码签名；仓库发布权限须受保护。</p><p>主控更新器默认解析最新正式 Release，并将发行包与校验清单固定到同一个标签；校验 SHA256 和包内版本后才进入部署。独立脚本可从仓库 <code>main</code> 拉取，因此源码写权限与 Release 发布权限均属于信任边界。脚本下载使用当前用户权限，验证后才调用 sudo；更新保留原配置和密钥，程序失败回退不撤销数据库迁移。</p><h2 id="数据与运行" tabindex="-1">数据与运行 <a class="header-anchor" href="#数据与运行" aria-label="Permalink to &quot;数据与运行&quot;">​</a></h2><p>通知配置使用 AES-256-GCM 加密。HTTPS / SMTP 验证 TLS，通知连接检查 DNS / IP，禁止重定向与代理绕过。事件、告警、通知通过事务去重，投递任务使用租约与状态检查。Agent 队列与状态绑定平台和服务器身份。</p><p>请求、队列、诊断输出与数据库维护均有边界限制。部署方负责保护数据库网络、备份、加密密钥和宿主机；依赖扫描不代表不存在漏洞。日志缺口、磁盘损坏、平台停机和通知重复的边界见 <a href="./docs/PRODUCTION.html">验收清单</a>。</p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("SECURITY.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const SECURITY = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  SECURITY as default
};
