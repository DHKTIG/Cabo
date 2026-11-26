const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const desktopDir = path.resolve(__dirname, '..');
const rootDir = path.resolve(desktopDir, '..');
const helperDir = path.join(desktopDir, '.electron');
const helperPath = path.join(helperDir, 'cabo-server.exe');

fs.mkdirSync(helperDir, { recursive: true });
const build = spawnSync('go', ['build', '-ldflags=-s -w -H windowsgui', '-o', helperPath, './cmd/cabo'], {
  cwd: rootDir,
  stdio: 'inherit',
});
if (build.status !== 0) {
  process.exit(build.status ?? 1);
}

const electron = spawn(require('electron'), [desktopDir], {
  cwd: desktopDir,
  stdio: 'inherit',
  env: {
    ...process.env,
    CABO_SERVER_PATH: helperPath,
    CABO_DEVTOOLS: '1',
  },
});
electron.once('exit', (code) => process.exit(code ?? 0));
