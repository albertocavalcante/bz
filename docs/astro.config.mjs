// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLlmsTxt from 'starlight-llms-txt';
import fs from 'node:fs';

const starlarkGrammar = JSON.parse(
  fs.readFileSync(new URL('./starlark.tmLanguage.json', import.meta.url), 'utf-8')
);

// https://astro.build/config
export default defineConfig({
  site: 'https://albertocavalcante.github.io',
  base: '/bz/',
  integrations: [
    starlight({
      title: 'bz',
      lastUpdated: true,
      expressiveCode: {
        shiki: {
          langs: [starlarkGrammar],
          langAlias: {
            'bzl': 'starlark',
            'bazel': 'starlark',
            'build': 'starlark',
          },
        },
      },
      description: 'Bazel module management CLI with Starlark-based configuration',
      social: {
        github: 'https://github.com/albertocavalcante/bz',
      },
      editLink: {
        baseUrl: 'https://github.com/albertocavalcante/bz/edit/main/docs/',
      },
      head: [
        {
          tag: 'meta',
          attrs: { property: 'og:type', content: 'website' },
        },
        {
          tag: 'meta',
          attrs: { name: 'twitter:card', content: 'summary_large_image' },
        },
      ],
      customCss: [
        './src/styles/custom.css',
      ],
      plugins: [
        starlightLlmsTxt({
          projectName: 'bz',
          description: 'bz is a Bazel module management CLI with Starlark-based configuration for syncing modules from registries to various destinations.',
          promote: ['index', 'getting-started', 'installation', 'configuration'],
          demote: ['faq', 'troubleshooting', 'contributing'],
        }),
      ],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Introduction', slug: '' },
            { label: 'Quick Start', slug: 'getting-started' },
            { label: 'Installation', slug: 'installation' },
          ],
        },
        {
          label: 'Configuration',
          items: [
            { label: 'Overview', slug: 'configuration' },
            { label: 'Starlark Config', slug: 'configuration/starlark', badge: { text: 'New', variant: 'tip' } },
            { label: 'Registries', slug: 'configuration/registries' },
            { label: 'Publishers', slug: 'configuration/publishers' },
            { label: 'Transforms', slug: 'configuration/transforms' },
          ],
        },
        {
          label: 'CLI Reference',
          items: [
            { label: 'Overview', slug: 'cli' },
            { label: 'mod search', slug: 'cli/mod-search' },
            { label: 'mod show', slug: 'cli/mod-show' },
            { label: 'mod sync', slug: 'cli/mod-sync', badge: { text: 'New', variant: 'tip' } },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Overview', slug: 'guides' },
            { label: 'Mirror BCR', slug: 'guides/mirror-bcr' },
            { label: 'Air-gapped Setup', slug: 'guides/air-gapped' },
          ],
        },
        {
          label: 'Resources',
          items: [
            { label: 'FAQ', slug: 'faq' },
            { label: 'Troubleshooting', slug: 'troubleshooting' },
            { label: 'Contributing', slug: 'contributing' },
          ],
        },
      ],
    }),
  ],
});
