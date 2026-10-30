import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"地理位置与公网 IP","description":"","frontmatter":{},"headers":[],"relativePath":"docs/GEOIP.md","filePath":"docs/GEOIP.md","lastUpdated":1793353928000}');
const _sfc_main = { name: "docs/GEOIP.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="地理位置与公网-ip" tabindex="-1">地理位置与公网 IP <a class="header-anchor" href="#地理位置与公网-ip" aria-label="Permalink to &quot;地理位置与公网 IP&quot;">​</a></h1><p>管理员打开侧栏「地理位置」，选择数据提供商并保存。服务器名称前显示按公网 IP 查询到的国家 / 地区旗帜；列表及详情只显示公网 IPv4 / IPv6。默认提供商为 geojs.io，无需密钥。选择「无」关闭国家 / 地区查询及旗帜，公网 IP 仍正常显示。</p><table tabindex="0"><thead><tr><th>选项</th><th>数据来源</th><th>配置</th></tr></thead><tbody><tr><td>无</td><td>关闭查询</td><td>无</td></tr><tr><td>MaxMind</td><td>本地 GeoLite2-Country MMDB</td><td>保存后点击「更新数据库」</td></tr><tr><td>ip-api.com</td><td>在线 API</td><td>免费接口使用 HTTP，每分钟最多 45 次，仅限非商业用途；可选 Pro Key 使用 HTTPS</td></tr><tr><td>geojs.io</td><td>在线 HTTPS API</td><td>无需密钥</td></tr><tr><td>ipinfo.io</td><td>在线 HTTPS API</td><td>可选 Token；填写后使用 IPinfo Lite，免 Token 接口受服务商配额限制</td></tr></tbody></table><p>在线查询从主控发出，只提交公网 IP。可选凭证使用现有 <code>ENCRYPTION_KEY</code> 加密存储，不返回浏览器；同一提供商下留空保留原凭证，勾选「清除已保存凭证」可移除，切换提供商会清空旧凭证。查询成功缓存 24 小时，失败短暂缓存并省略旗帜；服务故障不影响服务器状态和日志上报。</p><p>MaxMind 数据库从 <a href="https://github.com/Loyalsoldier/geoip" target="_blank" rel="noreferrer">Loyalsoldier GeoIP</a> 的 GeoLite2-Country 发布文件下载，与 Komari 的默认来源一致。该项目基于 MaxMind GeoLite2 数据构建。下载限制 32 MiB，校验 MMDB 格式和完整性后在事务中替换；失败保留旧库。数据库随 SQLite / PostgreSQL / MySQL 一起持久化和备份，无需额外数据目录。多实例在下一次查询时按数据库版本加载更新。</p><p>「测试 GeoIP」使用已保存的提供商查询输入的公网 IPv4 / IPv6，并展示旗帜、国家 / 地区名称和代码。设置尚未保存、选择「无」或 MaxMind 尚未下载时，测试按钮不可用。新增的 <code>010_geoip.sql</code> 迁移仅创建设置、数据库和心跳公网来源三张表，不改写原有事件或服务器配置。</p><p>NAT 地址的含义、外部探测与反代配置见 <a href="./AGENTS.html#上报设置与-ip-地址">Agent 手册</a>。Agent 与服务器 API 都过滤内网地址，兼容旧前端的 IP 字段也只返回公网地址。详情打开期间会跟随最新心跳刷新。SVG 旗帜使用 <a href="https://github.com/lipis/flag-icons" target="_blank" rel="noreferrer">flag-icons</a>，随前端打包，不依赖浏览器访问第三方旗帜服务；构建中包含其 <a href="https://github.com/xiuzhenzhi/SSH/blob/main/web/public/assets/flag-icons-LICENSE.txt" target="_blank" rel="noreferrer">MIT 许可证</a>。</p><h2 id="ssh-事件与首页统计" tabindex="-1">SSH 事件与首页统计 <a class="header-anchor" href="#ssh-事件与首页统计" aria-label="Permalink to &quot;SSH 事件与首页统计&quot;">​</a></h2><p>SSH 事件列表、事件详情和首页最近 5 条事件共用当前 GeoIP 提供商，显示来源国家 / 地区和旗帜。内网 / 保留地址单独标识；公网查询失败显示未知，事件仍可查看。归属地按当前数据源解析，不作为登录身份或安全结论。</p><p>首页「认证失败来源国家 TOP 5」统计近 24 小时失败事件，先按来源 IP 计数，再按国家 / 地区汇总。同一国家多个 IP 合并；未知归属的事件保留在总数及未定位计数中，不进入国家排名。选择「无」、MaxMind 未就绪或服务商不可用时显示对应空状态。</p><p>国家统计有独立的 5 秒短缓存，并发页面共享查询；单个页面取消不会中断其他页面，更换提供商会使旧统计失效。统计接口与字段见 <a href="./API.html">API 参考</a>。</p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("docs/GEOIP.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const GEOIP = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  GEOIP as default
};
