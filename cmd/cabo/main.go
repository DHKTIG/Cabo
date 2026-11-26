package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"cabo/internal/server"
)

var (
	logFile        *os.File
	requestLogging bool
)

// logf 写诊断日志。窗口一片白或者闪退时，看这里就知道卡在哪一步。
func logf(format string, args ...any) {
	if logFile == nil {
		return
	}
	fmt.Fprintf(logFile, "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

// logDirs 日志候选目录：用户缓存 → exe 同目录 → 临时目录。
func logDirs() []string {
	var out []string
	if d, err := os.UserCacheDir(); err == nil {
		out = append(out, filepath.Join(d, "Cabo"))
	}
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Dir(exe))
	}
	out = append(out, os.TempDir())
	return out
}

// setupLog 始终开启文件日志。库里有 log.Fatal 调用，不接管的话闪退时什么都不留下。
func setupLog(verbose bool) {
	for _, dir := range logDirs() {
		if os.MkdirAll(dir, 0o755) != nil {
			continue
		}
		f, err := os.Create(filepath.Join(dir, "cabo.log"))
		if err != nil {
			continue
		}
		logFile = f
		log.SetOutput(f)
		requestLogging = verbose
		logf("===== CABO 启动 =====")
		logf("日志位置：%s", f.Name())
		return
	}
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestLogging {
			logf("%s %s", r.Method, r.URL.RequestURI())
		}
		next.ServeHTTP(w, r)
	})
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			logf("!!! panic: %v\n%s", r, debug.Stack())
			where := "cabo.log（在程序目录或用户缓存目录）"
			if logFile != nil {
				where = logFile.Name()
			}
			reportError("CABO 崩溃", fmt.Sprintf("%v\n\n日志：%s", r, where))
		}
	}()

	browserMode := flag.Bool("browser", false, "用系统浏览器打开")
	serverMode := flag.Bool("server", false, "仅启动本地 HTTP 服务，供 Electron 调用")
	port := flag.Int("port", 0, "本地服务端口，0 表示自动分配")
	logTo := flag.Bool("log", false, "额外记录每个 HTTP 请求到 cabo.log")
	flag.Parse()

	setupLog(*logTo)

	addr := "127.0.0.1:0"
	if *port > 0 {
		addr = fmt.Sprintf("127.0.0.1:%d", *port)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logf("监听失败：%v", err)
		if *serverMode {
			fmt.Fprintln(os.Stderr, "监听失败:", err)
			os.Exit(1)
		}
		reportError("CABO 启动失败", "无法监听本地端口：\n"+err.Error())
		return
	}
	url := "http://" + ln.Addr().String()

	if logFile != nil {
		defer logFile.Close()
	}
	handler := withLogging(http.Handler(server.New()))
	logf("本地服务：%s", url)

	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
	}
	defer shutdown(srv)
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	if *serverMode {
		bindToParent()
		fmt.Printf("CABO_SERVER_URL=%s\n", url)
	} else {
		if *browserMode {
			logf("使用系统浏览器模式")
		}
		if err := openBrowser(url); err != nil {
			reportError("CABO", "请手动在浏览器打开：\n"+url)
			return
		}
		fmt.Println("游戏地址：", url)
		fmt.Println("关闭此窗口即可退出")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	select {
	case <-ctx.Done():
		logf("收到退出信号")
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			log.Println("服务异常：", err)
			reportError("CABO 服务异常", err.Error())
		}
	}
}

func shutdown(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logf("关闭服务失败：%v", err)
	}
}
