import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
export default defineConfig({ plugins: [vue(), {
  name: 'preserve-module-loading',
  transformIndexHtml: {
    order: 'post',
    handler: html => html.replace(/<script\b([^>]*)>/g, (tag, attributes: string) =>
      /\btype=["']module["']/.test(attributes) && !attributes.includes('data-cfasync=')
        ? tag.replace('<script', '<script data-cfasync="false"') : tag),
  },
}], server: { port: 5173, strictPort: true, proxy: { '/api': {target:'http://127.0.0.1:8080',ws:true} } } });
