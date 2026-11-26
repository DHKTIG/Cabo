const path = require('node:path');
const { ESLint } = require('eslint');

async function main() {
  const desktopDir = path.resolve(__dirname, '..');
  const rootDir = path.resolve(desktopDir, '..');
  const eslint = new ESLint({
    cwd: rootDir,
    overrideConfigFile: path.join(rootDir, 'eslint.config.mjs'),
  });
  const results = await eslint.lintFiles([
    'desktop/main.cjs',
    'desktop/scripts/**/*.cjs',
    'desktop/tests/**/*.mjs',
    'eslint.config.mjs',
    'internal/server/static/*.mjs',
  ]);
  const formatter = await eslint.loadFormatter('stylish');
  const output = formatter.format(results);
  if (output) process.stdout.write(output);
  process.exitCode = results.some((result) => result.errorCount > 0) ? 1 : 0;
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
