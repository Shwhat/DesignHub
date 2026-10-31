import { defineConfig, type Plugin } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// emptyOutDir clears dist on every build. The Go embed needs dist/.gitkeep
// to exist in a fresh checkout, so it is written back after each build.
function keepGitkeep(): Plugin {
  return {
    name: 'keep-gitkeep',
    apply: 'build',
    closeBundle() {
      writeFileSync(fileURLToPath(new URL('./dist/.gitkeep', import.meta.url)), '');
    },
  };
}

export default defineConfig({
  plugins: [svelte(), keepGitkeep()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsInlineLimit: 0,
    chunkSizeWarningLimit: 4000,
  },
  server: {
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
});
