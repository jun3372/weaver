# Database: db / mongodb

## db

Weaver's database client component, built on [GORM](https://github.com/go-gorm/gorm), automatically selects the MySQL / PostgreSQL / SQLite / ClickHouse dialect based on the DSN scheme. Simply declare `weaver.Ref[db.T]` for injection; connection establishment and graceful shutdown are managed by the framework lifecycle.

The component interface `db.T` provides three methods: `WithContext(ctx)` returns a `*gorm.DB` carrying the ctx (with automatic trace propagation), `DB(ctx)` is an equivalent alias, and `Transaction(ctx, fn, opts...)` executes `fn` within a transaction — if the callback returns an error, the transaction is rolled back automatically. An optional `*sql.TxOptions` can be passed to control the isolation level and read-only mode.

### Installation

```bash
go get github.com/jun3372/weaver/components/db
```

```go
import _ "github.com/jun3372/weaver/components/db" // anonymous import completes component registration
```

### Configuration

```yaml
db:
  Enabled: true
  DSN: "mysql://user:pass@tcp(127.0.0.1:3306)/app"
  GormConfig:
    Logger: {}
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the component is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `file:data/data.db?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL` | Connection string; the scheme determines the dialect, see the table below |
| `GormConfig` | `{}` | Passed through to `*gorm.Config` (logging, naming strategy, timeouts, etc.) |

Environment variable overrides: `WEAVER_DB_DSN`, `WEAVER_DB_ENABLED`, `WEAVER_DB_GORMCONFIG`.

The DSN supports both `scheme://...` and `scheme:...` forms, e.g. `sqlite://file:test.db`, `sqlite:test.db`, `mysql://...`; unknown schemes return `ErrInvalidDSN` / `ErrInvalidScheme`. Currently supported schemes:

| scheme | Dialect | Driver | Example |
|---|---|---|---|
| `sqlite3://` / `sqlite://` / `sqlite:` | SQLite | [glebarez/sqlite](https://github.com/glebarez/sqlite) (pure Go, no CGO) | `sqlite://file:test.db?mode=memory` |
| `file:` | SQLite | Same as above | `file:data/data.db?cache=shared&mode=rwc` |
| `mysql://` | MySQL | gorm.io/driver/mysql | `mysql://user:pass@tcp(127.0.0.1:3306)/db` |
| `postgres://` / `postgresql://` | PostgreSQL | gorm.io/driver/postgres | `postgres://user:pass@localhost:5432/db?sslmode=disable` |
| `clickhouse://` | ClickHouse | gorm.io/driver/clickhouse | `clickhouse://localhost:9000?username=gorm&password=gorm&database=gorm` |

### Basic Usage

```go
type userRepo struct {
    weaver.Implements[UserRepo]
    weaver.Ref[db.T]
}

func (u *userRepo) Find(ctx context.Context, id int64) (*User, error) {
    gdb, err := u.Ref().WithContext(ctx) // automatically carries ctx (trace propagation)
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

### Advanced Scenarios

#### Transactions

`Transaction` is a thin wrapper around GORM's `(*gorm.DB).Transaction`: returning `nil` from the callback commits, while returning an error rolls back automatically. The `tx` in the callback already carries the ctx, so chain operations directly on it.

```go
func (u *userRepo) Transfer(ctx context.Context, from, to int64, amount int) error {
    return u.Ref().Transaction(ctx, func(tx *gorm.DB) error {
        if err := tx.Model(&User{}).Where("id = ?", from).
            Update("balance", gorm.Expr("balance - ?", amount)).Error; err != nil {
            return err // rolls back automatically on error
        }
        return tx.Model(&User{}).Where("id = ?", to).
            Update("balance", gorm.Expr("balance + ?", amount)).Error
    }, &sql.TxOptions{Isolation: sql.LevelReadCommitted}) // optional: isolation level/read-only
}
```

#### AutoMigrate Table Creation

Once you have a `*gorm.DB` via `WithContext`, you can call GORM's migration capabilities, which is useful for syncing table structures from models during development or at startup:

```go
func (u *userRepo) Init(ctx context.Context) error {
    gdb, err := u.Ref().WithContext(ctx)
    if err != nil {
        return err
    }
    return gdb.AutoMigrate(&User{}, &Order{}) // creates tables if missing, adds columns if present
}
```

### Lifecycle & Errors

- `Init`: logs and returns directly when not enabled; selects the GORM dialector by DSN scheme and calls `gorm.Open`, returning an error on failure. Note that `gorm.Open` does not Ping, and drivers like SQLite connect lazily — if you need a strong validation at startup, execute a `SELECT 1` in your business component's `Start`
- `Shutdown`: closes the underlying `*sql.DB` connection pool

```go
gdb, err := u.Ref().WithContext(ctx)
if errors.Is(err, db.ErrNotEnabled) {
    // db component not enabled, fall back to a degraded path (e.g. read-only cache)
}
// Other sentinel errors: db.ErrNotReady (not ready), db.ErrInvalidDSN, db.ErrInvalidScheme
```

## mongodb

Weaver's MongoDB client component, built on [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver). Declare `weaver.Ref[mongodb.T]` to inject a `*mongo.Client`; connection establishment, Ping health checks, and graceful shutdown are managed by the framework lifecycle. The component interface has a single method: `Client(ctx) (*mongo.Client, error)`.

### Installation

```bash
go get github.com/jun3372/weaver/components/mongodb
```

```go
import _ "github.com/jun3372/weaver/components/mongodb"
```

### Configuration

```yaml
mongodb:
  Enabled: true
  DSN: "mongodb://user:pass@localhost:27017/app?replicaSet=rs0"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the component is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `mongodb://localhost:27017` | Connection string; supports URI parameters for replica sets, authentication, read/write preference, etc. |

Environment variable overrides: `WEAVER_MONGODB_DSN`, `WEAVER_MONGODB_ENABLED`.

### Basic Usage

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
            return nil, nil // no match
        }
        return nil, err
    }
    return &user, nil
}
```

### Advanced Scenarios

#### CRUD and bson tags

Model fields should have explicit `bson` tags, which do not conflict with GORM's `gorm` tags:

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
    // Update (Upsert): col.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": user},
    //   options.Update().SetUpsert(true)); Delete: col.DeleteOne(ctx, bson.M{"_id": user.ID})
    return nil
}
```

#### Aggregate Queries

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
    // process doc
}
return cur.Err()
```
#### Session Transactions

Use Session transactions (v2 API) for atomic multi-document operations: call `StartSession` and wrap the callback with `WithTransaction`; errors trigger an automatic abort, and success commits automatically. Transactions require a replica set or sharded cluster; a standalone `standalone` deployment is not supported.

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
        return nil, err // rolls back automatically on error
    }
    _, err := users.UpdateOne(sc, bson.M{"_id": to}, bson.M{"$inc": bson.M{"balance": amount}})
    return nil, err
})
return err
```

### Lifecycle & Errors

- `Init`: logs and returns directly when not enabled; after `mongo.Connect`, performs a `Ping` with a 5-second timeout (`readpref.Primary()`); on failure, it automatically calls `Disconnect` and returns an error
- `Shutdown`: `Disconnect` releases the connection

```go
cli, err := u.Ref().Client(ctx)
if errors.Is(err, mongodb.ErrNotEnabled) {
    // mongodb component not enabled, fall back to a degraded path
}
// Other sentinel errors: mongodb.ErrNotReady
```

Note: `mongo.Connect` uses lazy connection, so business errors often surface on the first operation; a query with no match always returns `mongo.ErrNoDocuments` — check it with `errors.Is` rather than comparing error strings directly.

## Further Reading

- [Components Overview](/en/guide/components)
- [Configuration](/en/guide/config)
