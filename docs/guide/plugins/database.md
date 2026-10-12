# 数据库:db / mongodb

## db

Weaver 的数据库客户端组件,基于 [GORM](https://github.com/go-gorm/gorm),按 DSN scheme 自动选择 MySQL / PostgreSQL / SQLite / ClickHouse 方言。声明 `weaver.Ref[db.T]` 即可注入,连接建立与优雅关闭由框架生命周期托管。

组件接口 `db.T` 提供三个方法:`WithContext(ctx)` 返回已携带 ctx(自动透传 trace)的 `*gorm.DB`,`DB(ctx)` 为其等价别名,`Transaction(ctx, fn, opts...)` 在事务中执行 `fn`——回调返回 error 则自动回滚,可选传入 `*sql.TxOptions` 控制隔离级别与只读属性。

### 安装

```bash
go get github.com/jun3372/weaver/plugins/db
```

```go
import _ "github.com/jun3372/weaver/plugins/db" // 匿名导入完成组件注册
```

### 配置

```yaml
db:
  Enabled: true
  DSN: "mysql://user:pass@tcp(127.0.0.1:3306)/app"
  GormConfig:
    Logger: {}
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `file:data/data.db?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL` | 连接串,scheme 决定方言,见下表 |
| `GormConfig` | `{}` | 透传 `*gorm.Config`(日志、命名策略、超时等) |

环境变量覆盖:`WEAVER_DB_DSN`、`WEAVER_DB_ENABLED`、`WEAVER_DB_GORMCONFIG`。

DSN 兼容 `scheme://...` 与 `scheme:...` 两种形式,如 `sqlite://file:test.db`、`sqlite:test.db`、`mysql://...`;未知 scheme 返回 `ErrInvalidDSN` / `ErrInvalidScheme`。当前支持的 scheme:

| scheme | 方言 | 驱动 | 示例 |
|---|---|---|---|
| `sqlite3://` / `sqlite://` / `sqlite:` | SQLite | [glebarez/sqlite](https://github.com/glebarez/sqlite)(纯 Go,免 CGO) | `sqlite://file:test.db?mode=memory` |
| `file:` | SQLite | 同上 | `file:data/data.db?cache=shared&mode=rwc` |
| `mysql://` | MySQL | gorm.io/driver/mysql | `mysql://user:pass@tcp(127.0.0.1:3306)/db` |
| `postgres://` / `postgresql://` | PostgreSQL | gorm.io/driver/postgres | `postgres://user:pass@localhost:5432/db?sslmode=disable` |
| `clickhouse://` | ClickHouse | gorm.io/driver/clickhouse | `clickhouse://localhost:9000?username=gorm&password=gorm&database=gorm` |

### 基础用法

```go
type userRepo struct {
    weaver.Implements[UserRepo]
    weaver.Ref[db.T]
}

func (u *userRepo) Find(ctx context.Context, id int64) (*User, error) {
    gdb, err := u.Ref().WithContext(ctx) // 自动携带 ctx(trace 透传)
    if err != nil {
        return nil, err
    }
    var user User
    if err := gdb.First(&user, id).Error; err != nil {
        return nil, err
    }
    return &user, nil
}
```

### 进阶场景

#### 事务

`Transaction` 内部即 GORM 的 `(*gorm.DB).Transaction`:回调返回 `nil` 提交,返回 error 自动回滚。回调中的 `tx` 已携带 ctx,链式操作直接使用即可。

```go
func (u *userRepo) Transfer(ctx context.Context, from, to int64, amount int) error {
    return u.Ref().Transaction(ctx, func(tx *gorm.DB) error {
        if err := tx.Model(&User{}).Where("id = ?", from).
            Update("balance", gorm.Expr("balance - ?", amount)).Error; err != nil {
            return err // 出错自动回滚
        }
        return tx.Model(&User{}).Where("id = ?", to).
            Update("balance", gorm.Expr("balance + ?", amount)).Error
    }, &sql.TxOptions{Isolation: sql.LevelReadCommitted}) // 可选:隔离级别/只读
}
```

#### AutoMigrate 建表

经 `WithContext` 拿到 `*gorm.DB` 后即可调用 GORM 的迁移能力,适合开发期或启动时按模型同步表结构:

```go
func (u *userRepo) Init(ctx context.Context) error {
    gdb, err := u.Ref().WithContext(ctx)
    if err != nil {
        return err
    }
    return gdb.AutoMigrate(&User{}, &Order{}) // 不存在则建表,存在则补列
}
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;按 DSN scheme 选择 GORM dialector 后 `gorm.Open`,失败返回错误。注意 `gorm.Open` 不做 Ping,SQLite 等驱动惰性建连,需启动期强校验时可在业务组件 `Start` 中执行一条 `SELECT 1`
- `Shutdown`:关闭底层 `*sql.DB` 连接池

```go
gdb, err := u.Ref().WithContext(ctx)
if errors.Is(err, db.ErrNotEnabled) {
    // db 插件未启用,走降级逻辑(如只读缓存)
}
// 其余哨兵错误:db.ErrNotReady(未就绪)、db.ErrInvalidDSN、db.ErrInvalidScheme
```

## mongodb

Weaver 的 MongoDB 客户端组件,基于 [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver)。声明 `weaver.Ref[mongodb.T]` 即可注入 `*mongo.Client`,连接建立、Ping 探活与优雅关闭由框架生命周期托管。组件接口仅一个方法:`Client(ctx) (*mongo.Client, error)`。

### 安装

```bash
go get github.com/jun3372/weaver/plugins/mongodb
```

```go
import _ "github.com/jun3372/weaver/plugins/mongodb"
```

### 配置

```yaml
mongodb:
  Enabled: true
  DSN: "mongodb://user:pass@localhost:27017/app?replicaSet=rs0"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `mongodb://localhost:27017` | 连接串,支持副本集、认证、读写偏好等 URI 参数 |

环境变量覆盖:`WEAVER_MONGODB_DSN`、`WEAVER_MONGODB_ENABLED`。

### 基础用法

```go
type userRepo struct {
    weaver.Implements[UserRepo]
    weaver.Ref[mongodb.T]
}

func (u *userRepo) Find(ctx context.Context, id int64) (*User, error) {
    cli, err := u.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    col := cli.Database("app").Collection("users")
    var user User
    if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
        if errors.Is(err, mongo.ErrNoDocuments) {
            return nil, nil // 未命中
        }
        return nil, err
    }
    return &user, nil
}
```

### 进阶场景

#### CRUD 与 bson tag

模型字段建议显式标注 `bson` tag,与 GORM 的 `gorm` tag 互不冲突:

```go
type User struct {
    ID   int64  `bson:"_id"`
    Name string `bson:"name"`
    Age  int    `bson:"age"`
}

func (u *userRepo) Save(ctx context.Context, user *User) error {
    cli, _ := u.Ref().Client(ctx)
    col := cli.Database("app").Collection("users")
    if _, err := col.InsertOne(ctx, user); err != nil {
        return err
    }
    // 更新(Upsert):col.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": user},
    //   options.Update().SetUpsert(true));删除:col.DeleteOne(ctx, bson.M{"_id": user.ID})
    return nil
}
```

#### Aggregate 聚合查询

```go
cli, _ := u.Ref().Client(ctx)
col := cli.Database("app").Collection("users")
cur, err := col.Aggregate(ctx, []bson.M{
    {"$match": bson.M{"age": bson.M{"$gte": 18}}},
    {"$group": bson.M{"_id": "$name", "count": bson.M{"$sum": 1}}},
    {"$sort": bson.M{"count": -1}},
})
if err != nil {
    return err
}
defer cur.Close(ctx)
for cur.Next(ctx) {
    var doc bson.M
    if err := cur.Decode(&doc); err != nil {
        return err
    }
    // 处理 doc
}
return cur.Err()
```
#### Session 事务

多文档原子操作使用 Session 事务(v2 API):`StartSession` 后以 `WithTransaction` 包裹回调,出错自动 abort,成功自动 commit。事务要求副本集或分片集群,单机 `standalone` 不支持。

```go
cli, err := u.Ref().Client(ctx)
if err != nil {
    return err
}
sess, err := cli.StartSession()
if err != nil {
    return err
}
defer sess.EndSession(ctx)
users := cli.Database("app").Collection("users")
_, err = sess.WithTransaction(ctx, func(sc context.Context) (any, error) {
    if _, err := users.UpdateOne(sc, bson.M{"_id": from},
        bson.M{"$inc": bson.M{"balance": -amount}}); err != nil {
        return nil, err // 出错自动回滚
    }
    _, err := users.UpdateOne(sc, bson.M{"_id": to}, bson.M{"$inc": bson.M{"balance": amount}})
    return nil, err
})
return err
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;`mongo.Connect` 后以 5 秒超时 `Ping`(`readpref.Primary()`),失败时自动 `Disconnect` 并返回错误
- `Shutdown`:`Disconnect` 释放连接

```go
cli, err := u.Ref().Client(ctx)
if errors.Is(err, mongodb.ErrNotEnabled) {
    // mongodb 插件未启用,走降级逻辑
}
// 其余哨兵错误:mongodb.ErrNotReady
```

注意:`mongo.Connect` 采用惰性建连,业务错误多发生在首次操作时;查询未命中统一返回 `mongo.ErrNoDocuments`,请用 `errors.Is` 判断而非直接比对错误文本。

## 相关阅读

- [插件总览](/guide/plugins)
- [配置管理](/guide/config)
