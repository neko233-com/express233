package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/express233/internal/api"
	"github.com/neko233-com/express233/internal/store"
	"github.com/neko233-com/express233/internal/version"
)

func main() {
	showVer := flag.Bool("version", false, "print version and exit")
	addr := flag.String("addr", defaultListenAddr(), "listen address")
	dataDir := flag.String("data", defaultDataDir(), "data directory")
	flag.Parse()

	if *showVer {
		fmt.Println(version.String("express233-server"))
		os.Exit(0)
	}

	listen := normalizeListenAddr(*addr)

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	ensureServerYAML(*dataDir)

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	srv := api.New(st)
	log.Printf("%s", version.String("express233-server"))
	if err := warnIfPortBlocked(listen); err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s (data: %s)", listen, *dataDir)
	log.Printf("访问地址 = %s", browserURL(listen))
	if wd := api.DevWebDir(); wd != "" {
		log.Printf("static hot reload: %s (html/css/js)", wd)
	}
	log.Printf("default login: root / root")
	if err := http.ListenAndServe(listen, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

func defaultListenAddr() string {
	if a := os.Getenv("EXPRESS233_ADDR"); a != "" {
		return a
	}
	return "127.0.0.1:23380"
}

func defaultDataDir() string {
	if d := os.Getenv("EXPRESS233_DATA"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".express233-server")
}

// normalizeListenAddr 将 :port 转为 127.0.0.1:port，避免 Windows 上仅监听 IPv6 而 127.0.0.1 被其它程序占用。
func normalizeListenAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func browserURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func warnIfPortBlocked(listen string) error {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil
	}
	probe := net.JoinHostPort("127.0.0.1", port)
	conn, err := net.DialTimeout("tcp", probe, 400*time.Millisecond)
	if err != nil {
		return nil
	}
	_ = conn.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + probe + "/healthz")
	if err != nil {
		return fmt.Errorf(
			"127.0.0.1:%s 已被其他程序占用（常见: proxysss）；请关闭该程序或设置 EXPRESS233_ADDR=127.0.0.1:其他端口",
			port,
		)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok" {
		return fmt.Errorf("已有 express233-server 在 %s 运行", browserURL(listen))
	}
	return fmt.Errorf(
		"127.0.0.1:%s 已被其他程序占用（常见: proxysss）；请关闭该程序或设置 EXPRESS233_ADDR=127.0.0.1:其他端口",
		port,
	)
}

func ensureServerYAML(dataDir string) {
	path := filepath.Join(dataDir, "server.yaml")
	if _, err := os.Stat(path); err == nil {
		return
	}
	example := `servers: {}
`
	if b, err := os.ReadFile("configs/server.yaml.example"); err == nil {
		example = string(b)
	}
	_ = os.WriteFile(path, []byte(example), 0o644)
	fmt.Printf("created default server.yaml at %s\n", path)
}
