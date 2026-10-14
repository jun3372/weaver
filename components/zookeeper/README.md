# weaver components/zookeeper

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/zookeeper.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/zookeeper)

Weaver 的 ZooKeeper 客户端组件，基于 [go-zookeeper/zk](https://github.com/go-zookeeper/zk)。声明 `weaver.Ref[zookeeper.T]` 即可注入，会话建立与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/zookeeper
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/zookeeper"
```

## 配置

```yaml
zookeeper:
  Enabled: true
  DSN: "zk://localhost:2181"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `zk://localhost:2181` | 集群地址，多节点逗号分隔：`zk://host1:2181,host2:2181,host3:2181` |

环境变量覆盖：`WEAVER_ZOOKEEPER_DSN`、`WEAVER_ZOOKEEPER_ENABLED`。

## 使用示例

```go
type leaderElection struct {
    weaver.Implements[LeaderElection]
    weaver.Ref[zookeeper.T]
}

func (l *leaderElection) Register(ctx context.Context, path string) error {
    conn, err := l.Get().Client(ctx)
    if err != nil {
        return err
    }
    _, err = conn.Create(path, nil, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
    return err
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；解析 DSN 后 `zk.Connect`（5 秒会话超时，库内自动重试），并校验进入 `StateConnected` 状态，失败自动 `Close` 并返回错误
- `Shutdown`：`Close` 结束会话

## 完整教程

更多用法见文档站的[注册与配置中心教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/registry.md#zookeeper)。
