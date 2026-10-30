import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"产品范围与后续路线","description":"","frontmatter":{},"headers":[],"relativePath":"docs/ROADMAP.md","filePath":"docs/ROADMAP.md","lastUpdated":1793296541000}');
const _sfc_main = { name: "docs/ROADMAP.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="产品范围与后续路线" tabindex="-1">产品范围与后续路线 <a class="header-anchor" href="#产品范围与后续路线" aria-label="Permalink to &quot;产品范围与后续路线&quot;">​</a></h1><p>SSHGuard 提供 SSH 采集、登录审计、安全告警、通知投递和服务器管理。当前功能与范围见 <a href="./../README.html">README</a> 和 <a href="./AGENTS.html">Agent 手册</a>。</p><p>后续方向包括 MFA / SSO、用户禁用与角色管理、更多定制 PAM 日志、通知恢复消息、队列恢复工具和数据库归档。具体变更见 <a href="./../CHANGELOG.html">更新日志</a>。</p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("docs/ROADMAP.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const ROADMAP = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  ROADMAP as default
};
