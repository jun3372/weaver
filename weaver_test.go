package weaver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverConf(t *testing.T) {
	dir := func(t *testing.T, files ...string) string {
		t.Helper()
		d := t.TempDir()
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(d, f), []byte("a: 1"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return d
	}

	tests := []struct {
		name  string
		files [][]string
		want  string
	}{
		{
			name:  "无配置文件返回空",
			files: [][]string{{"readme.txt"}},
			want:  "",
		},
		{
			name:  "命中 weaver.yaml",
			files: [][]string{{"weaver.yaml"}},
			want:  "weaver.yaml",
		},
		{
			name:  "同目录 weaver 优先于 config",
			files: [][]string{{"config.yaml", "weaver.toml"}},
			want:  "weaver.toml",
		},
		{
			name:  "同目录同名时扩展名 yaml 优先",
			files: [][]string{{"weaver.json", "weaver.yaml"}},
			want:  "weaver.yaml",
		},
		{
			name: "目录优先于文件名: 前目录 config 胜过后续目录 weaver",
			files: [][]string{
				{"config.json"},
				{"weaver.yaml"},
			},
			want: "config.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths []string
			for _, files := range tt.files {
				paths = append(paths, dir(t, files...))
			}
			got := discoverConf(paths)
			want := ""
			if tt.want != "" {
				want = filepath.Join(paths[0], tt.want)
			}
			if got != want {
				t.Fatalf("discoverConf() = %q, want %q", got, want)
			}
		})
	}
}
