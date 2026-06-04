package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/neko233-com/express233/internal/api"
	"github.com/neko233-com/express233/internal/store"
	"github.com/neko233-com/express233/internal/version"
)

func main() {
	showVer := flag.Bool("version", false, "print version and exit")
	addr := flag.String("addr", ":23380", "listen address")
	dataDir := flag.String("data", defaultDataDir(), "data directory")
	flag.Parse()

	if *showVer {
		fmt.Println(version.String("express233-server"))
		os.Exit(0)
	}

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
	log.Printf("listening on %s (data: %s)", *addr, *dataDir)
	log.Printf("default login: root / root")
	if err := http.ListenAndServe(*addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

func defaultDataDir() string {
	if d := os.Getenv("EXPRESS233_DATA"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".express233-server")
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
