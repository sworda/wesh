import { defineConfig } from 'vite';
import { viteSingleFile } from 'vite-plugin-singlefile';

export default defineConfig({
  plugins: [viteSingleFile()], // 全量 inline；插件对 viteMajor>=8 自动走 codeSplitting:false
});
