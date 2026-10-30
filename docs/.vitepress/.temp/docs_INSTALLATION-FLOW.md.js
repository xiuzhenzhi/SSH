import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"主控安装流程","description":"","frontmatter":{},"headers":[],"relativePath":"docs/INSTALLATION-FLOW.md","filePath":"docs/INSTALLATION-FLOW.md","lastUpdated":1793353626000}');
const _sfc_main = { name: "docs/INSTALLATION-FLOW.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="主控安装流程" tabindex="-1">主控安装流程 <a class="header-anchor" href="#主控安装流程" aria-label="Permalink to &quot;主控安装流程&quot;">​</a></h1><p>目标：用户只有一台服务器、没有域名，也能完成主控安装并接入第一台 Agent。参考 <a href="https://github.com/komari-monitor/komari/blob/main/install-komari.sh" target="_blank" rel="noreferrer">Komari 的安装入口</a>，将安装成功定义为“能够开始采集”，而非仅启动一个网页。</p><h2 id="默认路径" tabindex="-1">默认路径 <a class="header-anchor" href="#默认路径" aria-label="Permalink to &quot;默认路径&quot;">​</a></h2><ol><li>执行安装命令，在菜单选择「1 安装主控」，确认应用端口，回车使用 8080。自动检测架构、下载校验、配置低权限服务与开机自启。</li><li>打开终端给出的 <code>http://IP:端口/#setup=口令</code>，无需填写域名、反向代理或证书。</li><li>创建管理员。默认 SQLite；外部数据库和连接测试放在可选的「存储设置」。初始化口令从链接读取，失败时可修改。</li><li>自动登录，进入服务器管理。新增服务器后直接生成对应操作系统的安装命令。</li><li>在目标机器执行命令，确认在线及采集状态，再验证一次真实登录与通知投递。</li></ol><h2 id="场景与处理" tabindex="-1">场景与处理 <a class="header-anchor" href="#场景与处理" aria-label="Permalink to &quot;场景与处理&quot;">​</a></h2><table tabindex="0"><thead><tr><th>场景</th><th>行为</th></tr></thead><tbody><tr><td>已有 Nginx / OpenResty / 宝塔</td><td>默认使用独立端口，保留现有服务；反代按需配置</td></tr><tr><td>8080 被占用</td><td>明确提示 <code>--port 8081</code> 等替代方式，退出后可重试</td></tr><tr><td>初始化链接丢失</td><td>管理菜单「8」或 <code>--setup-link</code>；仅交互终端显示口令</td></tr><tr><td>初始化时刷新页面</td><td>同源 sessionStorage 暂存口令，完成后清除</td></tr><tr><td>服务切换尚未完成</td><td>显示已保存状态并轮询就绪，支持重新检查；不重复创建管理员</td></tr><tr><td>外部数据库连接失败</td><td>显示可处理的错误，保留表单；拒绝覆盖非空数据库</td></tr><tr><td>主控 IP / NAT</td><td>Agent 命令使用管理员实际访问地址；提示确认目标机可达及端口映射</td></tr><tr><td>更新主控</td><td>菜单「9」或 <code>sh /opt/sshguard/update.sh</code> 下载最新正式版并校验；已安装时默认「9」，保留原配置、身份与数据</td></tr><tr><td>卸载</td><td>菜单「7」，默认保留数据；卸载后再次打开菜单默认「1」安装</td></tr></tbody></table><h2 id="传输与配置" tabindex="-1">传输与配置 <a class="header-anchor" href="#传输与配置" aria-label="Permalink to &quot;传输与配置&quot;">​</a></h2><p>默认原生安装显式启用 HTTP，命令中携带授权参数并持久保存到 Agent 配置。HTTP 登录、令牌和事件未加密，Agent 发行文件通过 HTTPS 从 GitHub 下载，页面和终端均说明；SHA256 仅用于完整性校验。HTTPS 严格校验证书，下载重定向仅允许 HTTPS，不自动降级。域名、HTTPS 与外部数据库不作为默认安装的前置条件。</p><p>直连模式不信任转发头，使用请求地址做同源检查和命令生成；反代继续使用显式 <code>PUBLIC_URL</code>。已有安装不自动切换协议、数据库、端口或管理员。</p><h2 id="验收" tabindex="-1">验收 <a class="header-anchor" href="#验收" aria-label="Permalink to &quot;验收&quot;">​</a></h2><ul><li><code>scripts/test-direct-flow.mjs</code>：在隔离临时目录启动真实 API / Agent，验证默认 SQLite、初始化鉴权、登录、IP 注册、心跳、事件、更新任务与重启持久化。</li><li>Go 测试：HTTP 显式授权、IPv4 / IPv6 地址、HTTPS 证书拒绝、两种协议的 WebSocket 断线和提交后确认丢失回退。</li><li><code>scripts/quick-start.test.mjs</code>：隔离 GitHub 与提权操作，实际下载夹具、校验与解压，覆盖最新版本、指定版本、架构、失败中止和更新入口。</li><li>原生 CI：在一次性 Linux VM 验证真实 systemd Agent 的直连安装、采集、幂等更新、平台更新及主控升级；直连组还验证菜单「1」拒绝重复安装、「9」下载更新、已安装时默认更新和配置保留。</li><li>安装与下载使用已发布的版本，验证范围见 <a href="./VALIDATION.html">验证记录</a>。</li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("docs/INSTALLATION-FLOW.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const INSTALLATIONFLOW = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  INSTALLATIONFLOW as default
};
