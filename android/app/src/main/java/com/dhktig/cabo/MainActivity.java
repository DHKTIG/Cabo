package com.dhktig.cabo;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.graphics.Color;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Toast;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.URI;
import java.net.URISyntaxException;
import java.security.SecureRandom;
import java.util.Locale;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Minimal Android shell for the CABO loopback-only Go server. */
public final class MainActivity extends Activity {
    private static final String SERVER_LIBRARY = "libcabo_server.so";
    private static final String SERVER_URL_MARKER = "CABO_SERVER_URL=";
    private static final long STARTUP_TIMEOUT_MS = 15_000L;
    private static final long MAX_LOG_BYTES = 64 * 1024L;

    private final Handler mainHandler = new Handler(Looper.getMainLooper());
    private final Pattern serverURLPattern = Pattern.compile(
            "(?m)^" + SERVER_URL_MARKER + "([^\\r\\n]+)\\r?$");

    private WebView webView;
    private Process serverProcess;
    private ExecutorService startupExecutor;
    private Future<?> startupTask;
    private File serverLog;
    private OutputStream serverLogWriter;
    private Thread logPump;
    private String apiToken;
    private volatile boolean destroyed;
    private String pageOrigin;
    private int pagePort;

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        webView = new WebView(this);
        webView.setBackgroundColor(Color.rgb(237, 240, 244));
        setContentView(webView);
        configureWebView();
        startServer();
    }

    private void configureWebView() {
        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setAllowFileAccess(false);
        settings.setAllowContentAccess(false);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
        settings.setSupportMultipleWindows(false);
        settings.setJavaScriptCanOpenWindowsAutomatically(false);
        settings.setMediaPlaybackRequiresUserGesture(true);
        settings.setBuiltInZoomControls(false);
        settings.setDisplayZoomControls(false);
        settings.setGeolocationEnabled(false);

        // No JavascriptInterface is registered: web content has no native bridge.
        webView.setWebViewClient(new LocalOnlyWebViewClient());
        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public boolean onCreateWindow(WebView view, boolean isDialog,
                    boolean isUserGesture, android.os.Message resultMsg) {
                return false;
            }
        });
    }

    private void startServer() {
        try {
            apiToken = randomToken();
            serverLog = new File(getCacheDir(), "cabo_server.log");
            File executable = new File(getApplicationInfo().nativeLibraryDir, SERVER_LIBRARY);
            if (!executable.isFile()) {
                throw new IOException("找不到本地服务：" + executable.getAbsolutePath());
            }

            ProcessBuilder builder = new ProcessBuilder(executable.getAbsolutePath(),
                    "-server", "-port", "0");
            builder.environment().put("CABO_API_TOKEN", apiToken);
            builder.redirectErrorStream(true);
            // File redirects are API 26+, so pump the merged child streams on API 24+.
            serverLogWriter = new FileOutputStream(serverLog);
            serverProcess = builder.start();
            logPump = new Thread(this::pumpServerLog, "cabo-server-log");
            logPump.setDaemon(true);
            logPump.start();

            startupExecutor = Executors.newSingleThreadExecutor();
            startupTask = startupExecutor.submit(() -> {
                try {
                    URI serverURL = awaitServerURL();
                    mainHandler.post(() -> {
                        if (destroyed) {
                            return;
                        }
                        try {
                            loadGame(serverURL);
                        } catch (URISyntaxException error) {
                            failStartup(getString(R.string.start_failed), error);
                        }
                    });
                } catch (Exception error) {
                    mainHandler.post(() -> failStartup(getString(R.string.start_failed), error));
                }
            });
        } catch (Exception error) {
            failStartup(getString(R.string.start_failed), error);
        }
    }

    private void pumpServerLog() {
        try (InputStream input = serverProcess.getInputStream();
                OutputStream output = serverLogWriter) {
            byte[] buffer = new byte[4096];
            int count;
            while ((count = input.read(buffer)) != -1) {
                output.write(buffer, 0, count);
                output.flush();
            }
        } catch (IOException ignored) {
            // Closing the streams during Activity teardown intentionally ends the pump.
        }
    }

    private URI awaitServerURL() throws Exception {
        long deadline = SystemClock.elapsedRealtime() + STARTUP_TIMEOUT_MS;
        while (SystemClock.elapsedRealtime() < deadline) {
            if (Thread.currentThread().isInterrupted()) {
                throw new InterruptedException("启动已取消");
            }
            Matcher matcher = serverURLPattern.matcher(readServerLog());
            if (matcher.find()) {
                return validateLoopbackURL(matcher.group(1).trim());
            }
            if (!isServerRunning()) {
                throw new IOException(getString(R.string.server_exited, exitCode()));
            }
            Thread.sleep(100L);
        }
        throw new IOException(getString(R.string.startup_timeout));
    }

    private String readServerLog() throws IOException {
        if (serverLog == null || !serverLog.isFile()) {
            return "";
        }
        long length = Math.min(serverLog.length(), MAX_LOG_BYTES);
        byte[] bytes = new byte[(int) length];
        int offset = 0;
        try (FileInputStream input = new FileInputStream(serverLog)) {
            while (offset < bytes.length) {
                int count = input.read(bytes, offset, bytes.length - offset);
                if (count < 0) {
                    break;
                }
                offset += count;
            }
        }
        return new String(bytes, 0, offset, "UTF-8");
    }

    private static URI validateLoopbackURL(String value) throws URISyntaxException {
        URI uri = new URI(value);
        int port = uri.getPort();
        if (!"http".equalsIgnoreCase(uri.getScheme())
                || !"127.0.0.1".equals(uri.getHost())
                || uri.getRawUserInfo() != null
                || port < 1 || port > 65535
                || uri.getRawQuery() != null || uri.getRawFragment() != null
                || (uri.getRawPath() != null && !uri.getRawPath().isEmpty()
                    && !"/".equals(uri.getRawPath()))) {
            throw new URISyntaxException(value, "服务地址不是受信任的 127.0.0.1 回环地址");
        }
        return uri;
    }

    private void loadGame(URI serverURL) throws URISyntaxException {
        validateLoopbackURL(serverURL.toString());
        pagePort = serverURL.getPort();
        String base = "http://127.0.0.1:" + pagePort;
        pageOrigin = base;
        webView.loadUrl(base + "/#token=" + Uri.encode(apiToken));
    }

    private boolean isSameOrigin(String value) {
        if (pageOrigin == null) {
            return false;
        }
        try {
            URI uri = new URI(value);
            return "http".equalsIgnoreCase(uri.getScheme())
                    && "127.0.0.1".equals(uri.getHost())
                    && uri.getPort() == pagePort
                    && uri.getRawUserInfo() == null;
        } catch (URISyntaxException error) {
            return false;
        }
    }

    private void failStartup(String title, Throwable error) {
        if (destroyed) {
            return;
        }
        String message = error.getMessage() == null ? error.toString() : error.getMessage();
        if (serverLog != null) {
            message += "\n日志：" + serverLog.getAbsolutePath();
        }
        Toast.makeText(this, title + "\n" + message, Toast.LENGTH_LONG).show();
        finish();
    }

    private static String randomToken() {
        SecureRandom random = new SecureRandom();
        byte[] bytes = new byte[32];
        random.nextBytes(bytes);
        StringBuilder token = new StringBuilder(bytes.length * 2);
        for (byte value : bytes) {
            token.append(String.format(Locale.ROOT, "%02x", value & 0xff));
        }
        return token.toString();
    }

    private boolean isServerRunning() {
        try {
            serverProcess.exitValue();
            return false;
        } catch (IllegalThreadStateException error) {
            return true;
        }
    }

    private int exitCode() {
        try {
            return serverProcess.exitValue();
        } catch (IllegalThreadStateException error) {
            return -1;
        }
    }

    private boolean waitForExit(Process process, long timeoutMs) {
        long deadline = SystemClock.elapsedRealtime() + timeoutMs;
        while (SystemClock.elapsedRealtime() < deadline) {
            try {
                process.exitValue();
                return true;
            } catch (IllegalThreadStateException ignored) {
                // Still running.
            }
            synchronized (process) {
                try {
                    process.wait(Math.min(100L, Math.max(1L,
                            deadline - SystemClock.elapsedRealtime())));
                } catch (InterruptedException error) {
                    Thread.currentThread().interrupt();
                    return false;
                }
            }
        }
        try {
            process.exitValue();
            return true;
        } catch (IllegalThreadStateException error) {
            return false;
        }
    }

    private void stopServer() {
        Process process = serverProcess;
        if (process == null) {
            return;
        }
        process.destroy();
        if (!waitForExit(process, 3_000L) && Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            process.destroyForcibly();
            waitForExit(process, 1_000L);
        }
    }

    @Override
    protected void onPause() {
        if (webView != null) {
            webView.onPause();
        }
        super.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (webView != null) {
            webView.onResume();
        }
    }

    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }

    @Override
    protected void onDestroy() {
        destroyed = true;
        if (startupTask != null) {
            startupTask.cancel(true);
        }
        if (webView != null) {
            webView.stopLoading();
            webView.clearHistory();
            webView.clearCache(true);
            webView.removeAllViews();
            webView.destroy();
            webView = null;
        }
        stopServer();
        if (logPump != null) {
            logPump.interrupt();
        }
        if (serverLogWriter != null) {
            try {
                serverLogWriter.close();
            } catch (IOException ignored) {
                // Already closed by the pump or teardown is best-effort.
            }
            serverLogWriter = null;
        }
        if (startupExecutor != null) {
            startupExecutor.shutdownNow();
            startupExecutor = null;
        }
        startupTask = null;
        mainHandler.removeCallbacksAndMessages(null);
        super.onDestroy();
    }

    private final class LocalOnlyWebViewClient extends WebViewClient {
        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            return request.getUrl() == null || !isSameOrigin(request.getUrl().toString());
        }

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, String url) {
            return !isSameOrigin(url);
        }
    }
}
