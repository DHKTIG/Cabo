import { createRequire } from 'node:module';

const require = createRequire(new URL('./desktop/package.json', import.meta.url));
const js = require('@eslint/js');
const globals = require('globals');

export default [
  {
    ignores: ['desktop/node_modules/**', 'desktop/.electron/**', 'artifacts/**', 'coverage*.out'],
  },
  js.configs.recommended,
  {
    files: ['**/*.{js,mjs,cjs}'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
    rules: {
      eqeqeq: ['error', 'always'],
      'no-debugger': 'error',
      'no-dupe-keys': 'error',
      'no-unreachable': 'error',
      'no-unused-vars': ['error', { args: 'none', caughtErrors: 'none' }],
    },
  },
];
