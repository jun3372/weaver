# weaver components/mongodb

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/mongodb.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/mongodb)

Weaver 的 MongoDB 客户端组件，基于 [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver)。声明 `weaver.Ref[mongodb.T]` 即可注入，连接建立、探活与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/mongodb
```

在主包匿名导入以完成组件注册（`weaver generate` 生成代码所在包）：

```go
import _ "github.com/jun3372/weaver/components/mongodb"
```

## 配置

```yaml
mongodb:
  Enabled: true
  DSN: "mongodb://localhost:27017"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `mongodb://localhost:27017` | MongoDB 连接串，支持副本集/认证等 URI 参数 |

环境变量覆盖（优先级高于配置文件）：`WEAVER_MONGODB_DSN`、`WEAVER_MONGODB_ENABLED`。

## 使用示例

```go
type userRepo struct {
    weaver.Implements[UserRepo]
    weaver.Ref[mongodb.T]
}

func (u *userRepo) Find(ctx context.Context, name string) (*User, error) {
    cli, err := u.Get().Client(ctx)
    if err != nil {
        return nil, err
    }
    col := cli.Database("app").Collection("users")
    // ...
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；`mongo.Connect` 后以 5 秒超时 `Ping`（`readpref.Primary()`），失败时自动 `Disconnect` 并返回错误
- `Shutdown`：`Disconnect` 释放连接

## 完整教程

更多用法见文档站的[数据库教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/database.md#mongodb)。
