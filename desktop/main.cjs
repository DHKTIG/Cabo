const { app, BrowserWindow, Menu, dialog } = require('electron');
const { spawn } = require('node:child_process');
const { randomBytes } = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const net = require('node:net');
const path = require('node:path');

let serverProcess;
let mainWindow;
let quitting = false;
let serverOutput = '';
let shutdownPromise;

function resolveServerPath() {
  if (process.env.CABO_SERVER_PATH) {
    return path.resolve(process.env.CABO_SERVER_PATH);
  }
  const candidates = [
    path.join(__dirname, '.electron', 'cabo-server.exe'),
    path.join(process.resourcesPath, 'cabo-server.exe'),
  ];
  return candidates.find((candidate) => fs.existsSync(candidate)) || candidates[0];
}

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

function probeServer(url, child) {
  return new Promise((resolve) => {
    const request = http.get(`${url}/healthz`, (response) => {
      response.resume();
      resolve(response.statusCode === 200);
    });
    request.setTimeout(500, () => request.destroy());
    request.once('error', () => resolve(false));
    request.once('close', () => {
      if (child.exitCode !== null) {
        resolve(false);
      }
    });
  });
}

async function waitForServer(url, child) {
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`本地服务已退出（${child.exitCode}）${serverOutput ? `：${serverOutput}` : ''}`);
    }
    if (await probeServer(url, child)) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error('等待本地服务超时');
}

function stopServer() {
  if (shutdownPromise) return shutdownPromise;
  const child = serverProcess;
  serverProcess = undefined;
  if (!child || child.exitCode !== null) return Promise.resolve();
  shutdownPromise = new Promise((resolve) => {
    const force = setTimeout(() => child.kill(), 5000);
    child.once('exit', () => {
      clearTimeout(force);
      resolve();
    });
    try {
      child.kill();
    } catch {
      clearTimeout(force);
      resolve();
    }
  });
  return shutdownPromise;
}

function focusWindow() {
  if (!mainWindow) {
    return;
  }
  if (mainWindow.isMinimized()) {
    mainWindow.restore();
  }
  mainWindow.show();
  mainWindow.focus();
}

function createWindow(url) {
  const origin = new URL(url).origin;
  const icon = path.join(__dirname, '..', 'cmd', 'cabo', 'cabo.ico');
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 860,
    minWidth: 780,
    minHeight: 560,
    center: true,
    title: 'CABO 卡波',
    backgroundColor: '#edf0f4',
    autoHideMenuBar: true,
    icon: fs.existsSync(icon) ? icon : undefined,
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });

  Menu.setApplicationMenu(null);
  mainWindow.once('ready-to-show', () => mainWindow.show());
  mainWindow.on('closed', () => {
    mainWindow = undefined;
  });
  mainWindow.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
  mainWindow.webContents.on('will-navigate', (event, target) => {
    if (new URL(target).origin !== origin) {
      event.preventDefault();
    }
  });
  if (process.env.CABO_DEVTOOLS === '1') {
    mainWindow.webContents.openDevTools({ mode: 'detach' });
  }
  mainWindow.loadURL(url);
}

async function start() {
  if (!app.requestSingleInstanceLock()) {
    app.quit();
    return;
  }
  app.setAppUserModelId('com.dhktig.cabo');
  app.on('second-instance', focusWindow);
  await app.whenReady();

  app.on('before-quit', (event) => {
    if (quitting) return;
    event.preventDefault();
    quitting = true;
    void stopServer().finally(() => app.quit());
  });
  app.on('window-all-closed', () => {
    if (quitting) return;
    quitting = true;
    void stopServer().finally(() => app.quit());
  });

  try {
    const executable = resolveServerPath();
    if (!fs.existsSync(executable)) {
      throw new Error(`找不到本地服务程序：${executable}`);
    }
    const port = await getFreePort();
    const baseURL = `http://127.0.0.1:${port}`;
    const token = randomBytes(32).toString('hex');
    const pageURL = `${baseURL}/#token=${token}`;
    serverOutput = '';
    serverProcess = spawn(executable, ['-server', '-port', String(port)], {
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env, CABO_API_TOKEN: token },
    });
    serverProcess.stdout.resume();
    serverProcess.stderr.on('data', (chunk) => {
      serverOutput = (serverOutput + chunk.toString()).slice(-4000);
    });
    serverProcess.once('error', (error) => {
      if (!quitting) {
        dialog.showErrorBox('CABO 启动失败', error.message);
        app.quit();
      }
    });
    serverProcess.once('exit', (code) => {
      if (!quitting && mainWindow) {
        dialog.showErrorBox('CABO 服务已退出', `本地服务意外退出（${code}）。\n${serverOutput}`);
        app.quit();
      }
    });

    await waitForServer(baseURL, serverProcess);
    createWindow(pageURL);
  } catch (error) {
    quitting = true;
    await stopServer();
    dialog.showErrorBox('CABO 启动失败', error.message);
    app.quit();
  }
}

start();
