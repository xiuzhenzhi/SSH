import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"参与开发","description":"","frontmatter":{},"headers":[],"relativePath":"CONTRIBUTING.md","filePath":"CONTRIBUTING.md","lastUpdated":1793353928000}');
const _sfc_main = { name: "CONTRIBUTING.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="参与开发" tabindex="-1">参与开发 <a class="header-anchor" href="#参与开发" aria-label="Permalink to &quot;参与开发&quot;">​</a></h1><p>先阅读 <a href="./docs/ROADMAP.html">产品路线图</a> 和 <a href="./docs/DEVELOPMENT.html">开发文档</a>。提交应围绕一个明确问题，说明触发场景、行为变化与验证结果。</p><ul><li>修改 API/数据存储时同时考虑 SQLite、MySQL 和 PostgreSQL；新增迁移分别编写三套 SQL，并说明升级风险。SQLite 测试默认使用临时数据库，其他数据库通过独立测试 DSN 验证。</li><li>修改 Agent 时检查 Linux/Windows、amd64/arm64；涉及日志格式时附脱敏样本和解析测试。</li><li>不提交真实服务器信息、令牌、Cookie、密码、证书私钥、.env 或本地数据库文件。</li><li>修改安装/升级脚本时说明目标系统、用户权限、现有配置保留行为及失败恢复。</li><li>安装、菜单、下载方式、API 或功能行为变化时，同步 README、对应功能文档、文档总览和更新日志；当前命令示例与版本文件一致，历史验证记录保留原版本。更新器从 <code>main</code> 拉取的独立脚本必须可单文件运行，不能依赖旧目录里的辅助文件。</li><li>已发布标签和二进制不可覆盖，修复使用新版本。</li></ul><p>提交前至少运行 <code>go test ./...</code>、<code>go vet ./...</code> 和 <code>npm run build</code>；相关变更执行数据库集成测试或 Agent 生命周期测试。不要把交叉编译成功描述为目标系统实机验收完成。</p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("CONTRIBUTING.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const CONTRIBUTING = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  CONTRIBUTING as default
};
