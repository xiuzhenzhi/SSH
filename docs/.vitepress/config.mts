import { defineConfig } from 'vitepress';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { tokenize } from './search.mjs';

const root = fileURLToPath(new URL('../../', import.meta.url));
const version = readFileSync(new URL('../../VERSION', import.meta.url), 'utf8').trim();

export default defineConfig({
  lang: 'zh-CN',
  title: 'SSHGuard',
  description: 'SSH 登录审计、安全告警与服务器管理。SSHGuard 官方使用文档。',
  base: '/SSH/',
  srcDir: '..',
  srcExclude: ['.git/**', '.local/**', 'node_modules/**', 'web/**', 'dist/**', 'cmd/**', 'internal/**', 'migrations/**', 'scripts/**', 'deploy/**', '.github/**', 'docs/.vitepress/**'],
  rewrites: { 'docs/index.md': 'index.md' },
  outDir: fileURLToPath(new URL('../../.local/docs-site/dist', import.meta.url)),
  cacheDir: fileURLToPath(new URL('../../.local/docs-site/cache', import.meta.url)),
  lastUpdated: true,
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/SSH/sshguard.svg' }],
    ['meta', { name: 'theme-color', content: '#3563e9' }],
  ],
  vite: { publicDir: root + 'docs/public' },
  markdown: { lineNumbers: false },
  themeConfig: {
    logo: '/sshguard.svg',
    siteTitle: 'SSHGuard 文档',
    nav: [
      { text: '快速开始', link: '/docs/QUICKSTART' },
      { text: '使用指南', link: '/docs/README' },
      { text: '开发与 API', link: '/docs/DEVELOPMENT' },
      { text: 'v' + version, items: [
        { text: '发布说明', link: '/docs/RELEASE-NOTES' },
        { text: '更新日志', link: '/CHANGELOG' },
        { text: '下载发行包', link: 'https://github.com/xiuzhenzhi/SSH/releases/latest' },
      ] },
    ],
    sidebar: [
      { text: '开始使用', items: [
        { text: '文档总览', link: '/docs/README' },
        { text: '一键安装与更新', link: '/docs/QUICKSTART' },
        { text: '安装与接入流程', link: '/docs/INSTALLATION-FLOW' },
        { text: 'Agent 安装与管理', link: '/docs/AGENTS' },
        { text: '常见问题', link: '/docs/TROUBLESHOOTING' },
      ] },
      { text: '部署与运维', collapsed: false, items: [
        { text: '原生与手工部署', link: '/docs/INSTALL-NATIVE' },
        { text: 'Docker Compose', link: '/docs/INSTALL-COMPOSE' },
        { text: '反向代理与 HTTPS', link: '/docs/REVERSE-PROXY' },
        { text: '升级、备份与恢复', link: '/docs/OPERATIONS' },
        { text: '生产验收', link: '/docs/PRODUCTION' },
      ] },
      { text: '功能指南', collapsed: false, items: [
        { text: '告警与通知', link: '/docs/ALERTING' },
        { text: '地理位置与公网 IP', link: '/docs/GEOIP' },
        { text: '登录防护', link: '/docs/LOGIN-SECURITY' },
      ] },
      { text: '开发与项目', collapsed: false, items: [
        { text: '开发与测试', link: '/docs/DEVELOPMENT' },
        { text: 'API 参考', link: '/docs/API' },
        { text: 'HTTP 与 WebSocket', link: '/docs/TRANSPORT' },
        { text: '文档站维护', link: '/docs/DOCS-SITE' },
        { text: '构建与发布', link: '/docs/RELEASING' },
        { text: '验证记录', link: '/docs/VALIDATION' },
        { text: '更新日志', link: '/CHANGELOG' },
        { text: '参与开发', link: '/CONTRIBUTING' },
        { text: '安全策略', link: '/SECURITY' },
        { text: '产品范围', link: '/docs/ROADMAP' },
      ] },
    ],
    outline: { level: [2, 3], label: '本页目录' },
    search: {
      provider: 'local',
      options: {
        miniSearch: { options: Object.assign({}, { tokenize }) },
        locales: { root: { translations: {
        button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
        modal: { noResultsText: '没有找到相关内容', resetButtonTitle: '清空搜索',
          footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' } },
      } } } },
    },
    socialLinks: [{ icon: 'github', link: 'https://github.com/xiuzhenzhi/SSH' }],
    editLink: { pattern: 'https://github.com/xiuzhenzhi/SSH/edit/main/:path', text: '在 GitHub 上编辑此页' },
    lastUpdated: { text: '最后更新', formatOptions: { dateStyle: 'medium' } },
    docFooter: { prev: '上一篇', next: '下一篇' },
    sidebarMenuLabel: '文档导航',
    darkModeSwitchLabel: '切换外观',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    returnToTopLabel: '返回顶部',
    notFound: { title: '页面未找到', quote: '这个页面可能已移动，请通过导航或搜索查找文档。', linkLabel: '返回首页', linkText: '返回文档首页' },
    footer: { message: '自托管 SSH 登录审计与安全告警', copyright: 'SSHGuard · 文档与源码同步维护' },
  },
});
