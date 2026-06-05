package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RunPullBatch 按清单批量拉取：每行 server_id[,dest_dir]。
func RunPullBatch(opts PullOptions, listPath string) error {
	opts = MergePullOptions(opts)
	f, err := os.Open(listPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	lineNo := 0
	var firstErr error
	ok, fail := 0, 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		sid := strings.TrimSpace(parts[0])
		dest := opts.DestDir
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			dest = strings.TrimSpace(parts[1])
		}
		row := opts
		row.ServerID = sid
		row.DestDir = dest
		fmt.Printf("==> pull server_id=%s dest=%s\n", sid, dest)
		if err := RunPull(row); err != nil {
			fmt.Fprintf(os.Stderr, "   failed: %v\n", err)
			fail++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ok++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	fmt.Printf("batch done: ok=%d fail=%d\n", ok, fail)
	if fail > 0 {
		return firstErr
	}
	return nil
}
