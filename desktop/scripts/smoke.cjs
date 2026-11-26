const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs');
const http = require('node:http');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');

const desktopDir = path.resolve(__dirname, '..');
const rootDir = path.resolve(desktopDir, '..');
const helperDir = path.join(desktopDir, '.electron');
const helperPath = path.join(helperDir, 'cabo-server.exe');
const screenshotPath = path.join(os.tmpdir(), process.env.CABO_EXE_PATH ? 'cabo-portable-smoke.png' : 'cabo-electron-smoke.png');

function getFreePort() {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
  });
}

function fetchJson(url) {
  return new Promise((resolve, reject) => {
    const request = http.get(url, (response) => {
      let body = '';
      response.setEncoding('utf8');
      response.on('data', (chunk) => {
        body += chunk;
      });
      response.on('end', () => {
        try {
          resolve(JSON.parse(body));
        } catch (error) {
          reject(error);
        }
      });
    });
    request.setTimeout(500, () => request.destroy());
    request.once('error', reject);
  });
}

async function waitForPage(port) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    try {
      const pages = await fetchJson(`http://127.0.0.1:${port}/json/list`);
      const page = pages.find((item) => item.type === 'page' && item.url.startsWith('http://127.0.0.1:'));
      if (page) {
        return page;
      }
    } catch {
      // Electron may still be starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error('Timed out waiting for Electron game page');
}

async function waitForDocument(cdp) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    try {
      const result = await cdp.send('Runtime.evaluate', {
        expression: `location.protocol === 'http:' && location.hostname === '127.0.0.1' && document.readyState === 'complete'`,
        returnByValue: true,
      });
      if (result.result.value === true) return;
    } catch (error) {
      if (!String(error.message).includes('Execution context was destroyed')) throw error;
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error('Timed out waiting for the game document');
}

function createCdp(webSocketUrl) {
  const socket = new WebSocket(webSocketUrl);
  let nextId = 0;
  const pending = new Map();
  const listeners = new Map();
  socket.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);
    if (!message.id) {
      for (const listener of listeners.get(message.method) || []) listener(message.params);
      return;
    }
    if (!pending.has(message.id)) return;
    const { resolve, reject, method } = pending.get(message.id);
    pending.delete(message.id);
    if (message.error) {
      reject(new Error(`${method}: ${message.error.message}`));
    } else {
      resolve(message.result);
    }
  });
  const ready = new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true });
    socket.addEventListener('error', reject, { once: true });
  });
  return {
    ready,
    send(method, params = {}) {
      const id = ++nextId;
      return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject, method });
        socket.send(JSON.stringify({ id, method, params }));
      });
    },
    on(method, listener) {
      const entries = listeners.get(method) || [];
      entries.push(listener);
      listeners.set(method, entries);
    },
    close() {
      socket.close();
    },
  };
}

async function waitForProcessExit(child, timeout = 5000) {
  if (child.exitCode !== null) return;
  await new Promise((resolve) => {
    const timer = setTimeout(resolve, timeout);
    child.once('exit', () => {
      clearTimeout(timer);
      resolve();
    });
  });
}

async function main() {
  const portablePath = process.env.CABO_EXE_PATH ? path.resolve(process.env.CABO_EXE_PATH) : null;
  if (!portablePath) {
    fs.mkdirSync(helperDir, { recursive: true });
    const serverBuild = spawnSync('go', ['build', '-ldflags=-s -w -H windowsgui', '-o', helperPath, './cmd/cabo'], {
      cwd: rootDir,
      stdio: 'inherit',
    });
    if (serverBuild.status !== 0) {
      process.exit(serverBuild.status ?? 1);
    }
  }

  const port = await getFreePort();
  let electron;
  let profileDir;
  let stagedDir;
  if (portablePath) {
    if (!fs.existsSync(portablePath)) {
      throw new Error(`Portable EXE not found: ${portablePath}`);
    }
    stagedDir = fs.mkdtempSync(path.join(os.tmpdir(), 'cabo-portable-smoke-'));
    const staged = path.join(stagedDir, 'cabo.exe');
    fs.copyFileSync(portablePath, staged);
    profileDir = fs.mkdtempSync(path.join(os.tmpdir(), 'cabo-smoke-profile-'));
    electron = spawn(staged, [
      `--remote-debugging-port=${port}`,
      `--user-data-dir=${profileDir}`,
    ], {
      cwd: stagedDir,
      stdio: 'inherit',
    });
  } else {
    const electronDist = path.dirname(require('electron'));
    const electronPackage = JSON.parse(fs.readFileSync(path.join(electronDist, '..', 'package.json'), 'utf8'));
    const runtimeDir = path.join(os.tmpdir(), 'cabo-electron-smoke-runtime');
    const runtimeVersion = path.join(runtimeDir, '.version');
    const runtimeExecutable = path.join(runtimeDir, 'electron.exe');
    if (!fs.existsSync(runtimeVersion) || fs.readFileSync(runtimeVersion, 'utf8') !== electronPackage.version) {
      fs.rmSync(runtimeDir, { recursive: true, force: true });
      fs.cpSync(electronDist, runtimeDir, { recursive: true });
      fs.writeFileSync(runtimeVersion, electronPackage.version);
    }
    electron = spawn(runtimeExecutable, [desktopDir, '--disable-gpu', `--remote-debugging-port=${port}`], {
      cwd: desktopDir,
      stdio: 'inherit',
      env: {
        ...process.env,
        CABO_SERVER_PATH: helperPath,
      },
    });
  }
  let cdp;
  let cleanupError;
  try {
    const page = await waitForPage(port);
    cdp = createCdp(page.webSocketDebuggerUrl);
    await cdp.ready;
    const runtimeEvents = [];
    cdp.on('Runtime.exceptionThrown', (params) => runtimeEvents.push({ type: 'exception', ...params }));
    cdp.on('Runtime.consoleAPICalled', (params) => runtimeEvents.push({ type: 'console', ...params }));
    cdp.on('Log.entryAdded', (params) => runtimeEvents.push({ type: 'log', ...params }));
    await cdp.send('Runtime.enable');
    await cdp.send('Log.enable');
    await waitForDocument(cdp);
    await cdp.send('Page.enable');
    await cdp.send('Page.bringToFront');
    const evaluation = await cdp.send('Runtime.evaluate', {
      awaitPromise: true,
      returnByValue: true,
      expression: `(async () => {
        const sleep = ms => new Promise(r => setTimeout(r, ms));
        const button = text => [...document.querySelectorAll('button')].find(b => b.textContent.trim() === text);
        const until = async fn => {
          for (let i = 0; i < 200; i++) {
            const value = fn();
            if (value) return value;
            await sleep(100);
          }
          const wipe = document.querySelector('.wipe');
          const debug = {
            text: document.body.innerText.slice(0, 1200),
            href: location.origin + location.pathname + location.search,
            visibility: document.visibilityState,
            ready: window.__CABO_READY__,
            startClass: document.querySelector('#startOverlay')?.className,
            appClass: document.querySelector('#app')?.className,
            wipe: wipe ? {
              className: wipe.className,
              style: wipe.getAttribute('style'),
              animations: wipe.getAnimations().map((animation) => ({ state: animation.playState, time: animation.currentTime })),
            } : null,
            requests: window.__caboRequests || [],
            errors: window.__caboErrors || [],
          };
          throw new Error('Timed out waiting for game UI: ' + JSON.stringify(debug));
        };
        await until(() => window.__CABO_READY__ === true && button('开始游戏'));
        window.__caboRequests = [];
        window.__caboErrors = [];
        window.addEventListener('error', (event) => window.__caboErrors.push(event.message));
        window.addEventListener('unhandledrejection', (event) => window.__caboErrors.push(String(event.reason)));
        const originalFetch = window.fetch;
        window.fetch = async (...args) => {
          try {
            const response = await originalFetch(...args);
            window.__caboRequests.push({ url: String(args[0]), status: response.status });
            return response;
          } catch (error) {
            window.__caboRequests.push({ url: String(args[0]), error: String(error) });
            throw error;
          }
        };
        button('开始游戏').click();
        await until(() => document.querySelector('.wipe') || button('看牌'));
        const wipe = document.querySelector('.wipe');
        if (wipe) await sleep(80);
        const wipeStyle = wipe ? getComputedStyle(wipe) : null;
        const transition = wipe ? {
          clipPath: wipeStyle.clipPath,
          willChange: wipeStyle.willChange,
        } : { skipped: document.visibilityState === 'hidden' };
        await until(() => button('看牌'));
        button('看牌').click();
        await until(() => document.querySelectorAll('#myHand .card').length === 4 && document.querySelector('#actions .hint')?.textContent.includes('选 2 张'));
        const cards = [...document.querySelectorAll('#myHand .card')];
        cards[0].click();
        await sleep(120);
        cards[1].click();
        const confirm = await until(() => {
          const candidate = button('确认');
          return candidate && !candidate.disabled ? candidate : null;
        });
        confirm.click();
        const danger = await until(() => button('喊 CABO'));
        const primary = button('摸牌堆');
        const pick = element => {
          const style = getComputedStyle(element);
          return {
            text: element.textContent.trim(),
            color: style.color,
            backgroundColor: style.backgroundColor,
            backgroundImage: style.backgroundImage,
            borderColor: style.borderColor,
            width: style.width,
            height: style.height,
          };
        };
        const rect = element => {
          const box = element.getBoundingClientRect();
          return { left: box.left, top: box.top, right: box.right, bottom: box.bottom };
        };
        const initial = {
          danger: pick(danger),
          primary: pick(primary),
          action: document.querySelector('#actions').innerText,
          layout: {
            viewport: { width: innerWidth, height: innerHeight },
            scrollWidth: document.documentElement.scrollWidth,
            danger: rect(danger),
            primary: rect(primary),
          },
        };

        return { ...initial, transition };
      })()`,
    });
    if (evaluation.exceptionDetails || !evaluation.result.value) {
      throw new Error(JSON.stringify({ details: evaluation.exceptionDetails || evaluation.result, runtimeEvents }));
    }
    const result = evaluation.result.value;
    if (result.layout.scrollWidth > result.layout.viewport.width) {
      throw new Error(`Default layout overflows horizontally: ${result.layout.scrollWidth} > ${result.layout.viewport.width}`);
    }
    if (!result.transition.skipped && (result.transition.clipPath !== 'none' || result.transition.willChange !== 'transform')) {
      throw new Error(`Start transition should use a compositor transform: ${JSON.stringify(result.transition)}`);
    }
    for (const name of ['danger', 'primary']) {
      const box = result.layout[name];
      if (box.left < 0 || box.right > result.layout.viewport.width) {
        throw new Error(`${name} button is outside the default viewport: ${JSON.stringify(box)}`);
      }
    }
    await cdp.send('Emulation.setDeviceMetricsOverride', {
      width: 780,
      height: 560,
      deviceScaleFactor: 1,
      mobile: false,
    });
    await new Promise((resolve) => setTimeout(resolve, 300));
    const compact = await cdp.send('Runtime.evaluate', {
      returnByValue: true,
      expression: `(() => {
        const danger = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === '喊 CABO');
        const primary = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === '摸牌堆');
        return {
          viewport: { width: innerWidth, height: innerHeight },
          scrollWidth: document.documentElement.scrollWidth,
          dangerVisible: !!danger && getComputedStyle(danger).display !== 'none',
          primaryVisible: !!primary && getComputedStyle(primary).display !== 'none',
        };
      })()`,
    });
    await cdp.send('Emulation.clearDeviceMetricsOverride');
    if (compact.exceptionDetails || !compact.result.value) {
      throw new Error(JSON.stringify(compact.exceptionDetails || compact.result));
    }
    if (compact.result.value.scrollWidth > compact.result.value.viewport.width) {
      throw new Error(`Compact layout overflows horizontally: ${JSON.stringify(compact.result.value)}`);
    }
    if (!compact.result.value.dangerVisible || !compact.result.value.primaryVisible) {
      throw new Error(`Compact layout lost action buttons: ${JSON.stringify(compact.result.value)}`);
    }

    const discardEvaluation = await cdp.send('Runtime.evaluate', {
      awaitPromise: true,
      returnByValue: true,
      expression: `(async () => {
        const sleep = ms => new Promise(r => setTimeout(r, ms));
        const button = text => [...document.querySelectorAll('button')].find(b => b.textContent.trim() === text);
        const until = async fn => {
          for (let i = 0; i < 200; i++) {
            const value = fn();
            if (value) return value;
            await sleep(100);
          }
          throw new Error('Timed out waiting for discard UI: ' + document.body.innerText.slice(0, 1200));
        };
        await until(() => button('摸牌堆'));
        button('摸牌堆').click();
        await until(() => button('弃掉'));
        button('弃掉').click();
        await until(() => document.querySelector('.discard-holder .card .front'));
        const top = document.querySelector('.discard-holder .card .front');
        const style = getComputedStyle(top);
        const { showCaboBanner } = await import('/effects.mjs');
        showCaboBanner(0);
        await until(() => document.querySelector('.cabo-callout'));
        const callout = document.querySelector('.cabo-callout');
        const calloutStyle = getComputedStyle(callout);
        const calloutRect = callout.getBoundingClientRect();
        const playerRect = document.getElementById('meBar').getBoundingClientRect();
        callout.style.animation = 'none';
        callout.style.opacity = '1';
        return {
          discard: {
            count: Number(document.querySelector('#discardCount').textContent),
            fanCount: document.querySelectorAll('.discard-fan').length,
            backgroundColor: style.backgroundColor,
            backgroundImage: style.backgroundImage,
            backdropFilter: style.backdropFilter,
          },
          cabo: {
            color: calloutStyle.color,
            fontFamily: calloutStyle.fontFamily,
            fontSize: calloutStyle.fontSize,
            fontStyle: calloutStyle.fontStyle,
            fontWeight: calloutStyle.fontWeight,
            width: calloutRect.width,
            height: calloutRect.height,
            bottom: calloutRect.bottom,
            playerTop: playerRect.top,
            abovePlayerBar: calloutRect.bottom <= playerRect.top,
            borderColor: calloutStyle.borderColor,
            boxShadow: calloutStyle.boxShadow,
            backgroundImage: calloutStyle.backgroundImage,
            textStrokeWidth: calloutStyle.getPropertyValue('-webkit-text-stroke-width'),
            oldCount: document.querySelectorAll('.cabo-balloon').length,
          },
        };
      })()`,
    });
    if (discardEvaluation.exceptionDetails || !discardEvaluation.result.value) {
      throw new Error(JSON.stringify(discardEvaluation.exceptionDetails || discardEvaluation.result));
    }
    const { discard, cabo } = discardEvaluation.result.value;
    if (discard.count < 1 || discard.fanCount !== 0) {
      throw new Error(`Discard pile should show only its top card: ${JSON.stringify(discard)}`);
    }
    if (discard.backgroundColor !== 'rgb(255, 255, 255)' || discard.backdropFilter !== 'none') {
      throw new Error(`Discard top card should be opaque white: ${JSON.stringify(discard)}`);
    }
    if (cabo.oldCount !== 0 || cabo.color !== 'rgb(139, 120, 154)' || cabo.backgroundImage === 'none' || cabo.textStrokeWidth !== '0px') {
      throw new Error(`CABO callout should use liquid glass: ${JSON.stringify(cabo)}`);
    }
    if (!cabo.fontFamily.includes('Comic Sans MS') || cabo.fontStyle !== 'italic' || cabo.fontWeight !== '700') {
      throw new Error(`CABO callout typography is incorrect: ${JSON.stringify(cabo)}`);
    }
    if (!cabo.borderColor.includes('255, 255, 255') || !cabo.boxShadow.includes('217, 106, 114')) {
      throw new Error(`CABO callout should have a shallow red glow: ${JSON.stringify(cabo)}`);
    }
    if (!cabo.abovePlayerBar || cabo.width > 280 || Number.parseFloat(cabo.fontSize) > 54) {
      throw new Error(`CABO callout should be smaller and above player info: ${JSON.stringify(cabo)}`);
    }

    const screenshot = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(screenshotPath, Buffer.from(screenshot.data, 'base64'));
    console.log(JSON.stringify({ ...result, compact: compact.result.value, discard, cabo }, null, 2));
    console.log(`Screenshot: ${screenshotPath}`);
    void cdp.send('Browser.close').catch(() => {});
  } finally {
    cdp?.close();
    if (electron.exitCode === null) {
      if (process.platform === 'win32') {
        spawnSync('taskkill', ['/pid', String(electron.pid), '/t', '/f'], { stdio: 'ignore' });
      } else {
        electron.kill();
      }
    }
    await waitForProcessExit(electron);
    if (stagedDir) {
      fs.rmSync(stagedDir, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
      if (fs.existsSync(stagedDir)) cleanupError = new Error(`Failed to remove smoke directory: ${stagedDir}`);
    }
    if (profileDir) {
      fs.rmSync(profileDir, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
      if (fs.existsSync(profileDir)) cleanupError = new Error(`Failed to remove smoke profile: ${profileDir}`);
    }
  }
  if (cleanupError) throw cleanupError;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
