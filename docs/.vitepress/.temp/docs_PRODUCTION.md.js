import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"生产部署与分批验收","description":"","frontmatter":{},"headers":[],"relativePath":"docs/PRODUCTION.md","filePath":"docs/PRODUCTION.md","lastUpdated":1793353928000}');
const _sfc_main = { name: "docs/PRODUCTION.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="生产部署与分批验收" tabindex="-1">生产部署与分批验收 <a class="header-anchor" href="#生产部署与分批验收" aria-label="Permalink to &quot;生产部署与分批验收&quot;">​</a></h1><p>代码具备真实采集与通知链路，不代表任何目标环境已经自动完成上线。请记录以下检查结果；不要仅根据心跳在线或 CI 绿灯判断覆盖完整。</p><h2 id="部署基础" tabindex="-1">部署基础 <a class="header-anchor" href="#部署基础" aria-label="Permalink to &quot;部署基础&quot;">​</a></h2><p>中央平台推荐专用 Linux 主机，起步可用 2 vCPU / 2 GiB，实际规格由事件量、数据库规模及保留期决定；这不是 70 台负载承诺。先用代表性流量测量 CPU、RSS、事件入库延迟和通知队列。数据库访问限制在平台或隔离网络；独立模式只开放实际主控端口，HTTPS 反代模式仅公开站点端口。使用维护中的 OS 和数据库版本。</p><p>公网域名使用可信 TLS 证书，开启时间同步，验证 Nginx 的 WebSocket upgrade 与证书自动续期。备份数据库、api.env / .env、ENCRYPTION_KEY、证书；在隔离数据库中完成一次恢复演练。配置独立的服务存活、磁盘、数据库空间监控。</p><h2 id="先-2–3-台-再-10-台-再全部" tabindex="-1">先 2–3 台，再 10 台，再全部 <a class="header-anchor" href="#先-2–3-台-再-10-台-再全部" aria-label="Permalink to &quot;先 2–3 台，再 10 台，再全部&quot;">​</a></h2><p>首批至少覆盖 systemd Linux、Alpine OpenRC 和 Windows OpenSSH 中实际使用的种类。</p><ol><li>一键安装成功，重启主机后自动恢复；服务为低权限账号，凭证不可被服务用户修改。</li><li>控制台显示采集正常；真实成功/失败 SSH 登录均能看到准确账号、来源 IP、认证方式和时间。</li><li>各实际使用通知渠道测试送达；随后用真实登录触发规则，核对告警与接收消息。</li><li>断开 Agent 到平台网络，生成多个事件，重启 Agent，再恢复网络；确认队列清空、事件不重复。</li><li>执行本机日志轮转，跨轮转登录无缺口；验证日志权限在重启和轮转后仍有效。</li><li>同一来源重复失败达到阈值；可信 CIDR 和分组过滤符合预期；确认告警后状态更新。</li><li>停止 Agent，离线告警按阈值出现；恢复后标记已恢复。模拟日志不可读，采集异常可见。</li><li>更新 Agent 不换身份、保留队列；下载校验失败不停止旧服务；失败启动恢复旧程序。</li><li>撤销凭证后，已建立的 WSS 和后续 HTTPS 均不能再上报；只读用户不能管理渠道、规则或凭证。</li><li>持续观察至少一个完整日志轮转及备份周期，记录每分钟事件量、处理延迟、磁盘与内存，再扩大规模。</li></ol><p>主控升级同样先在测试环境验证：菜单「9」或最新独立更新器保留配置、身份和数据；校验失败应维持旧服务，启动失败应恢复旧程序。升级后核对运行版本、网页初始化状态、登录、资产、事件、通知及 <code>/opt/sshguard/docs</code>。数据库迁移不自动回退，具体入口见 <a href="./QUICKSTART.html#更新中央平台">一键更新</a>。</p><h2 id="保留与容量" tabindex="-1">保留与容量 <a class="header-anchor" href="#保留与容量" aria-label="Permalink to &quot;保留与容量&quot;">​</a></h2><p>默认事件保留 30 天、已完成告警/投递保留 90 天、审计保留 180 天。可用 EVENT_RETENTION_DAYS、ALERT_RETENTION_DAYS、AUDIT_RETENTION_DAYS 配置，范围 30–3650 天。每 10 分钟分批清理，每表每次最多 1000 条；数据增长快于清理时须监控积压并制定数据库归档方案。</p><p>待投递、正在投递和失败待处理任务及其关联告警不会自动删除。处置长期失败任务、关闭不必要的规则并设置合理冷却，避免通知积压。Agent 本地队列有 10,000 个事件上限；满时停止读取，不会扩展为无界内存或无限磁盘队列。</p><h2 id="已知边界" tabindex="-1">已知边界 <a class="header-anchor" href="#已知边界" aria-label="Permalink to &quot;已知边界&quot;">​</a></h2><ul><li>Windows 仅监控 OpenSSH，不覆盖 RDP 或所有 Windows 安全登录；Linux 解析 OpenSSH 标准 Accepted / Failed 记录，第三方定制 PAM 文本可能无法解析。</li><li>日志被删除/压缩、copytruncate 竞态、磁盘损坏、断网超过源日志保留期均可能导致缺口；本平台不是完整 SIEM 或操作系统日志备份。</li><li>不保证外部通知 exactly-once。需要现有监控发现中央平台自身宕机；不包含短信、电话或商业服务商代付账号。</li><li>当前没有 MFA、SSO、用户禁用/改角色界面、内置数据库主从切换、通用远程执行或主控定时自动升级。Agent 支持管理员触发受限的固定版本更新；主控更新由运维在服务器执行。</li><li>amd64 的自动化集成验证与 arm64 交叉编译不等同于所有发行版/硬件实机验收；真实服务商账号、公网 ACME 与你的 70 台服务器需自行按清单验证。</li></ul><p>完整已执行测试与对应 CI 见 <a href="./VALIDATION.html">验证记录</a>。安装见 <a href="./QUICKSTART.html">快速开始</a>，通知配置见 <a href="./ALERTING.html">告警手册</a>，失败处理见 <a href="./TROUBLESHOOTING.html">故障排查</a>。</p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("docs/PRODUCTION.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const PRODUCTION = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  PRODUCTION as default
};
