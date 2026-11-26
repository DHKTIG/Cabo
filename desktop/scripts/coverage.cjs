const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const desktopDir = path.resolve(__dirname, '..');
const rootDir = path.resolve(desktopDir, '..');
const profilePath = path.join(rootDir, 'coverage.out');
const thresholds = new Map([
  ['cabo/internal/engine', 90],
  ['cabo/internal/game', 90],
  ['cabo/internal/storage', 90],
  ['cabo/internal/ai', 85],
  ['cabo/internal/server', 85],
]);

function run(args) {
  const result = spawnSync(args.command, args.args, { cwd: rootDir, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

try {
  const testArgs = ['test', '-coverprofile=' + profilePath, './cmd/...', './internal/...'];
  if (!process.argv.includes('--no-race')) testArgs.splice(1, 0, '-race');
  run({ command: 'go', args: testArgs });

  const packages = new Map();
  let totalStatements = 0;
  let totalCovered = 0;
  const lines = fs.readFileSync(profilePath, 'utf8').trim().split(/\r?\n/).slice(1);
  for (const line of lines) {
    const match = /^(.+?):\d+\.\d+,\d+\.\d+\s+(\d+)\s+(\d+)$/.exec(line);
    if (!match) continue;
    const pkg = path.posix.dirname(match[1].replaceAll('\\', '/'));
    const statements = Number(match[2]);
    const covered = Number(match[3]) > 0 ? statements : 0;
    const current = packages.get(pkg) || { statements: 0, covered: 0 };
    current.statements += statements;
    current.covered += covered;
    packages.set(pkg, current);
    totalStatements += statements;
    totalCovered += covered;
  }

  let failed = false;
  for (const [pkg, minimum] of thresholds) {
    const stats = packages.get(pkg) || { statements: 0, covered: 0 };
    const percent = stats.statements === 0 ? 0 : stats.covered * 100 / stats.statements;
    console.log(`${pkg}: ${percent.toFixed(1)}% (minimum ${minimum}%)`);
    if (percent < minimum) failed = true;
  }
  const total = totalStatements === 0 ? 0 : totalCovered * 100 / totalStatements;
  console.log(`total: ${total.toFixed(1)}% (minimum 80%)`);
  if (total < 80) failed = true;
  if (failed) process.exitCode = 1;
} finally {
  fs.rmSync(profilePath, { force: true });
}
