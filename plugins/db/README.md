# weaver plugins/db

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/plugins/db.svg)](https://pkg.go.dev/github.com/jun3372/weaver/plugins/db)

Weaver 的数据库客户端组件，基于 [GORM](https://github.com/go-gorm/gorm)，按 DSN 自动选择 MySQL / PostgreSQL / SQLite 方言。声明 `weaver.Ref[db.T]` 即可注入，连接、事务与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/plugins/db
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/plugins/db"
```

## 配置

```yaml
db:
  Enabled: true
  DSN: "mysql://user:pass@tcp(127.0.0.1:3306)/app"
  GormConfig:
    Logger: {}
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `file:data/data.db?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL` | 连接串，scheme 决定方言，见下表 |
| `GormConfig` | `{}` | 透传 `gorm.Config` |

环境变量覆盖：`WEAVER_DB_DSN`、`WEAVER_DB_ENABLED`。

### 支持的 DSN scheme

| scheme | 方言 | 示例 |
|---|---|---|
| `sqlite3://` / `sqlite://` / `sqlite:` | SQLite | `sqlite://file:test.db?mode=memory` |
| `file:` | SQLite | `file:data/data.db?cache=shared&mode=rwc` |
| `mysql://` | MySQL | `mysql://user:pass@tcp(127.0.0.1:3306)/db` |
| `postgres://` / `postgresql://` | PostgreSQL | `postgres://user:pass@localhost:5432/db?sslmode=disable` |
| `clickhouse://` / `clickhouse:` | ClickHouse | `clickhouse://localhost:9000?username=gorm&password=gorm&database=gorm` |

## 使用示例

```go
type userRepo struct {
    weaver.Implements[UserRepo]
    weaver.Ref[db.T]
}

func (u *userRepo) Find(ctx context.Context, id int64) (*User, error) {
    gdb, err := u.Ref().WithContext(ctx) // 自动携带 ctx（trace 透传）
    if err != nil {
        return nil, err
    }
    var user User
    if err := gdb.First(&user, id).Error; err != nil {
        return nil, err
    }
    return &user, nil
}

func (u *userRepo) Transfer(ctx context.Context, from, to int64) error {
    return u.Ref().Transaction(ctx, func(tx *gorm.DB) error {
        // tx 已携带 ctx
        return nil
    })
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；按 DSN scheme 选择 GORM dialector 后 `gorm.Open`
- `Shutdown`：关闭底层 `*sql.DB` 连接池

## 完整教程

更多用法见文档站的[数据库教程](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/database.md#db)。
