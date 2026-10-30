import babel from '@rolldown/plugin-babel';
import react, { reactCompilerPreset } from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The built app is embedded into the Go binary (internal/site/dist) and served from any sub-path.
export default defineConfig({
  base: './',
  plugins: [react(), babel({ presets: [reactCompilerPreset()] })],
  build: { outDir: '../internal/site/dist', emptyOutDir: true, assetsDir: 'assets' },
});
