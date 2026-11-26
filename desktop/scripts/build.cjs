const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { build } = require('electron-builder');

const desktopDir = path.resolve(__dirname, '..');
const rootDir = path.resolve(desktopDir, '..');
const helperDir = path.join(desktopDir, '.electron');
const helperPath = path.join(helperDir, 'cabo-server.exe');

async function main() {
  fs.mkdirSync(helperDir, { recursive: true });
  const serverBuild = spawnSync('go', ['build', '-ldflags=-s -w -H windowsgui', '-o', helperPath, './cmd/cabo'], {
    cwd: rootDir,
    stdio: 'inherit',
  });
  if (serverBuild.status !== 0) {
    process.exit(serverBuild.status ?? 1);
  }

  await build({
    projectDir: desktopDir,
  });

  const built = path.join(rootDir, 'artifacts', 'electron', 'cabo.exe');
  const target = path.join(rootDir, 'artifacts', 'app', 'cabo.exe');
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.copyFileSync(built, target);
  console.log(`CABO Portable EXE: ${target}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
