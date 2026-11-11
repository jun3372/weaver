package generate

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRoot 是相对本包测试工作目录的仓库根路径(go test 的工作目录是包目录)。
const repoRoot = "../.."

// snapshotCase 描述一个「包 → 生成产物」快照用例。
type snapshotCase struct {
	pattern string // 传给 Generate 的包模式(相对仓库根)
	file    string // 生成产物路径(相对仓库根)
	golden  string // golden 快照路径(相对本测试目录)
}

// 用 examples/hello 作为快照对象:它同时覆盖「主组件(weaver.Main)」与
// 「普通组件接口」两条注册路径,以及两者的 `var _ 接口 = (*实现)(nil)` 断言。
//
// 这是清理生成器死代码前的安全网:产物只要发生任何字节级变化(多一行、
// 少一行、改一个标识符、调整 import 顺序)测试都会失败。
var helloSnapshots = []snapshotCase{
	{
		pattern: "./examples/hello",
		file:    "examples/hello/weaver_gen.go",
		golden:  "testdata/golden/hello_main.weaver_gen.golden",
	},
	{
		pattern: "./examples/hello/chat",
		file:    "examples/hello/chat/weaver_gen.go",
		golden:  "testdata/golden/hello_chat.weaver_gen.golden",
	},
	{
		pattern: "./examples/hello/user",
		file:    "examples/hello/user/weaver_gen.go",
		golden:  "testdata/golden/hello_user.weaver_gen.golden",
	},
}

// TestGenerateSnapshotExamplesHello 对 examples/hello 的生成结果做 golden 快照比对。
//
// 生成器直接写回仓库内的真实文件,因此这里在生成前先备份原始内容,并在
// 测试结束(含失败路径)时恢复,避免测试失败把工作树弄脏。
func TestGenerateSnapshotExamplesHello(t *testing.T) {
	backup := make(map[string][]byte, len(helloSnapshots))
	for _, sc := range helloSnapshots {
		b, err := os.ReadFile(filepath.Join(repoRoot, sc.file))
		if err != nil {
			t.Fatalf("读取 %s: %v", sc.file, err)
		}
		backup[sc.file] = b
	}
	t.Cleanup(func() {
		for file, content := range backup {
			if err := os.WriteFile(filepath.Join(repoRoot, file), content, 0o644); err != nil {
				t.Errorf("恢复 %s: %v", file, err)
			}
		}
	})

	for _, sc := range helloSnapshots {
		if err := Generate(repoRoot, []string{sc.pattern}, Options{BuildTags: "ignoreWeaverGen"}); err != nil {
			t.Fatalf("Generate(%s): %v", sc.pattern, err)
		}
	}

	for _, sc := range helloSnapshots {
		got, err := os.ReadFile(filepath.Join(repoRoot, sc.file))
		if err != nil {
			t.Fatalf("读取生成产物 %s: %v", sc.file, err)
		}
		want, err := os.ReadFile(sc.golden)
		if err != nil {
			t.Fatalf("读取 golden %s: %v", sc.golden, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s 的生成产物与 golden 快照不一致\n--- got ---\n%s\n--- want ---\n%s", sc.file, got, want)
		}
	}
}

// TestSnapshotGoldenExists 防止 golden 快照被误删或清空:快照缺失时上面
// 的快照比对会因为 ReadFile 失败而报错,这里再显式断言文件存在且非空。
func TestSnapshotGoldenExists(t *testing.T) {
	for _, sc := range helloSnapshots {
		info, err := os.Stat(sc.golden)
		if err != nil {
			t.Fatalf("golden 快照 %s 缺失: %v", sc.golden, err)
		}
		if info.Size() == 0 {
			t.Fatalf("golden 快照 %s 为空", sc.golden)
		}
	}
}
