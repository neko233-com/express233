package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/neko233-com/express233/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "pull", "deploy":
		runPull(os.Args[1], os.Args[2:])
	case "pull-batch":
		runPullBatch(os.Args[2:])
	case "preview":
		runPreview(os.Args[2:])
	case "servers":
		runServers(os.Args[2:])
	case "versions":
		runVersions(os.Args[2:])
	case "config":
		runConfig(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "rollback":
		runRollback(os.Args[2:])
	case "diff":
		runDiff(os.Args[2:])
	case "version", "-v", "--version":
		cli.PrintVersion()
	case "upgrade":
		ver := "latest"
		if len(os.Args) > 2 {
			ver = os.Args[2]
		}
		fatal(cli.InstallOrSwitch(ver))
	case "install":
		ver := "latest"
		if len(os.Args) > 2 {
			ver = os.Args[2]
		}
		fatal(cli.InstallOrSwitch(ver))
	case "downgrade":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: express233-cli downgrade <version>"))
		}
		fatal(cli.InstallOrSwitch(os.Args[2]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func pullFlags(args []string) (cli.PullOptions, *flag.FlagSet) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	opts := cli.PullOptions{}
	fs.StringVar(&opts.ServerURL, "server", os.Getenv("EXPRESS233_SERVER"), "express233-server base URL")
	fs.StringVar(&opts.Project, "project", os.Getenv("EXPRESS233_PROJECT"), "project name")
	fs.StringVar(&opts.ServerID, "server-id", os.Getenv("EXPRESS233_SERVER_ID"), "server id (server.yaml key)")
	fs.StringVar(&opts.Token, "token", os.Getenv("EXPRESS233_TOKEN"), "pull token")
	fs.StringVar(&opts.Version, "version", "", "version (default: latest published)")
	fs.StringVar(&opts.DestDir, "dest", ".", "destination directory")
	fs.BoolVar(&opts.SkipHook, "skip-hook", false, "skip post_hook script")
	_ = fs.Parse(args)
	return opts, fs
}

func runPull(cmd string, args []string) {
	opts, _ := pullFlags(args)
	opts = cli.MergePullOptions(opts)
	if opts.ServerURL == "" || opts.Project == "" || opts.ServerID == "" || opts.Token == "" {
		fatal(fmt.Errorf("%s requires: --server --project --server-id --token (or ~/.express233/config.yaml)", cmd))
	}
	fatal(cli.RunPull(opts))
}

func runPullBatch(args []string) {
	fs := flag.NewFlagSet("pull-batch", flag.ExitOnError)
	list := fs.String("file", "", "batch file: one server_id per line, or server_id,dest_dir")
	opts := cli.PullOptions{}
	fs.StringVar(&opts.ServerURL, "server", os.Getenv("EXPRESS233_SERVER"), "server URL")
	fs.StringVar(&opts.Project, "project", os.Getenv("EXPRESS233_PROJECT"), "project")
	fs.StringVar(&opts.Token, "token", os.Getenv("EXPRESS233_TOKEN"), "token")
	fs.StringVar(&opts.Version, "version", "", "version")
	fs.StringVar(&opts.DestDir, "dest", ".", "default dest when line has only server_id")
	fs.BoolVar(&opts.SkipHook, "skip-hook", false, "skip post_hook")
	_ = fs.Parse(args)
	if *list == "" || opts.ServerURL == "" || opts.Project == "" || opts.Token == "" {
		fatal(fmt.Errorf("pull-batch requires: --file --server --project --token"))
	}
	fatal(cli.RunPullBatch(opts, *list))
}

func runRollback(args []string) {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	to := fs.String("to", "", "explicit version to deploy")
	steps := fs.Int("steps", 1, "how many published versions to step back when --to is empty")
	opts := cli.PullOptions{}
	fs.StringVar(&opts.ServerURL, "server", os.Getenv("EXPRESS233_SERVER"), "server URL")
	fs.StringVar(&opts.Project, "project", os.Getenv("EXPRESS233_PROJECT"), "project")
	fs.StringVar(&opts.ServerID, "server-id", os.Getenv("EXPRESS233_SERVER_ID"), "server id")
	fs.StringVar(&opts.Token, "token", os.Getenv("EXPRESS233_TOKEN"), "token")
	fs.StringVar(&opts.DestDir, "dest", ".", "dest")
	fs.BoolVar(&opts.SkipHook, "skip-hook", false, "skip post_hook")
	_ = fs.Parse(args)
	fatal(cli.RunRollback(opts, *to, *steps))
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	server := fs.String("server", os.Getenv("EXPRESS233_SERVER"), "server URL")
	token := fs.String("token", os.Getenv("EXPRESS233_TOKEN"), "token")
	project := fs.String("project", os.Getenv("EXPRESS233_PROJECT"), "project")
	serverID := fs.String("server-id", os.Getenv("EXPRESS233_SERVER_ID"), "server id")
	_ = fs.Parse(args)
	fatal(cli.RunDoctor(*server, *token, *project, *serverID))
}

func runVersions(args []string) {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)
	server := fs.String("server", os.Getenv("EXPRESS233_SERVER"), "server URL")
	project := fs.String("project", os.Getenv("EXPRESS233_PROJECT"), "project")
	token := fs.String("token", os.Getenv("EXPRESS233_TOKEN"), "token")
	_ = fs.Parse(args)
	cfg, _ := cli.LoadUserConfig()
	if *server == "" {
		*server = cfg.Server
	}
	if *project == "" {
		*project = cfg.Project
	}
	if *token == "" {
		*token = cfg.Token
	}
	if *server == "" || *project == "" || *token == "" {
		fatal(fmt.Errorf("versions requires: --server --project --token"))
	}
	fatal(cli.RunListVersions(*server, *project, *token))
}

func runConfig(args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("usage: express233-cli config show|set|init"))
	}
	switch args[0] {
	case "show":
		fatal(cli.PrintConfig())
	case "init":
		cfg := cli.UserConfig{
			Server:  os.Getenv("EXPRESS233_SERVER"),
			Token:   os.Getenv("EXPRESS233_TOKEN"),
			Project: os.Getenv("EXPRESS233_PROJECT"),
		}
		fatal(cli.SaveUserConfig(cfg))
		p, _ := cli.ConfigPath()
		fmt.Println("wrote", p)
	case "set":
		if len(args) != 3 {
			fatal(fmt.Errorf("usage: express233-cli config set <server|token|project|dest> <value>"))
		}
		cfg, err := cli.LoadUserConfig()
		fatal(err)
		switch args[1] {
		case "server":
			cfg.Server = args[2]
		case "token":
			cfg.Token = args[2]
		case "project":
			cfg.Project = args[2]
		case "dest":
			cfg.Dest = args[2]
		default:
			fatal(fmt.Errorf("unknown key %q", args[1]))
		}
		fatal(cli.SaveUserConfig(cfg))
	default:
		fatal(fmt.Errorf("unknown config subcommand %q", args[0]))
	}
}

func runServers(args []string) {
	fs := flag.NewFlagSet("servers", flag.ExitOnError)
	server := fs.String("server", os.Getenv("EXPRESS233_SERVER"), "server URL")
	token := fs.String("token", os.Getenv("EXPRESS233_TOKEN"), "token")
	_ = fs.Parse(args)
	if *server == "" || *token == "" {
		fatal(fmt.Errorf("servers requires: --server --token"))
	}
	fatal(cli.RunListServers(*server, *token))
}

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	server := fs.String("server", os.Getenv("EXPRESS233_SERVER"), "server URL")
	project := fs.String("project", os.Getenv("EXPRESS233_PROJECT"), "project")
	from := fs.String("from", "", "from version")
	to := fs.String("to", "", "to version")
	serverID := fs.String("server-id", os.Getenv("EXPRESS233_SERVER_ID"), "server id")
	token := fs.String("token", os.Getenv("EXPRESS233_TOKEN"), "token")
	_ = fs.Parse(args)
	if *server == "" || *project == "" || *from == "" || *to == "" || *serverID == "" || *token == "" {
		fatal(fmt.Errorf("diff requires: --server --project --from --to --server-id --token"))
	}
	fatal(cli.RunDiff(*server, *project, *from, *to, *serverID, *token))
}

func runPreview(args []string) {
	fs := flag.NewFlagSet("preview", flag.ExitOnError)
	server := fs.String("server", os.Getenv("EXPRESS233_SERVER"), "server URL")
	project := fs.String("project", "", "project")
	version := fs.String("version", "", "version (empty = latest published)")
	serverID := fs.String("server-id", "", "server id")
	token := fs.String("token", os.Getenv("EXPRESS233_TOKEN"), "token")
	_ = fs.Parse(args)
	if *server == "" || *project == "" || *serverID == "" || *token == "" {
		fatal(fmt.Errorf("preview requires: --server --project --server-id --token"))
	}
	fatal(cli.RunPreview(*server, *project, *version, *serverID, *token))
}

func usage() {
	fmt.Print(`express233-cli - 游戏逻辑服拉模式部署 CLI

一行部署（SSH 上执行）:
  EXPRESS233_SERVER=http://central:23380 EXPRESS233_TOKEN=xxx \
	express233-cli deploy -project mygame -server-id logic-042 -dest /opt/game/042

命令:
  deploy|pull   拉取已发布版本 + 按 server_id 替换配置 + post_hook
  preview       预览该 server_id 配置键 before -> after（不发版）
  servers       列出 server.yaml 中所有 server_id
  versions      列出项目已发布版本
  config        管理 ~/.express233/config.yaml
  doctor        检查中央服务 / token / server_id 是否就绪
  rollback      回滚到上一已发布版本（或 --to 指定版本）
  diff          对比两版本在 server_id 下的配置键差异（token）
  pull-batch    按文件批量拉取（每行 server_id 或 server_id,dest）
  upgrade|downgrade|install  CLI 自更新
  version

pull/deploy 参数:
  --server --project --server-id --token [--version] [--dest] [--skip-hook]

pull-batch:
  --file servers.csv --server URL --project NAME --token TOKEN

环境变量:
  EXPRESS233_SERVER / EXPRESS233_TOKEN / EXPRESS233_PROJECT / EXPRESS233_SERVER_ID

`)
}

func fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}