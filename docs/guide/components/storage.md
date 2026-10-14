# 存储:s3 / elasticsearch / meilisearch

## s3

Weaver 的 S3 兼容对象存储客户端组件,基于 [minio-go/v7](https://github.com/minio/minio-go)。一个组件同时覆盖 MinIO、AWS S3 及其他一切 S3 兼容存储。声明 `weaver.Ref[s3.T]` 即可注入 `*minio.Client`,client 初始化与 bucket 探活由框架生命周期托管。

### 安装

```bash
go get github.com/jun3372/weaver/components/s3
```

```go
import _ "github.com/jun3372/weaver/components/s3" // 匿名导入完成组件注册
```

### 配置

```yaml
s3:
  Enabled: true
  DSN: "s3://minioadmin:minioadmin@localhost:9000?secure=false"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `s3://minioadmin:minioadmin@localhost:9000?secure=false` | 格式 `s3://access:secret@host:port[/bucket]?secure=true\|false` |

DSN 细节:`bucket` 可选,配置后 `Init` 阶段执行 `BucketExists` 探活,不存在则报 `ErrBucketNotFound`;`secure` 未指定默认 `false`(本地 MinIO 场景,连 AWS S3 记得写 `secure=true`)。

环境变量覆盖:`WEAVER_S3_DSN`、`WEAVER_S3_ENABLED`。

### 基础用法

```go
type fileStore struct {
    weaver.Implements[FileStore]
    weaver.Ref[s3.T]
}

func (f *fileStore) Upload(ctx context.Context, object, path string) error {
    cli, err := f.Get().Client(ctx)
    if err != nil {
        return err
    }
    _, err = cli.FPutObject(ctx, "mybucket", object, path, minio.PutObjectOptions{})
    return err
}

func (f *fileStore) Download(ctx context.Context, object string) ([]byte, error) {
    cli, err := f.Get().Client(ctx)
    if err != nil {
        return nil, err
    }
    obj, err := cli.GetObject(ctx, "mybucket", object, minio.GetObjectOptions{})
    if err != nil {
        return nil, err
    }
    defer obj.Close()
    return io.ReadAll(obj)
}
```

### 进阶场景

#### 创建 bucket

```go
cli, _ := f.Get().Client(ctx)

exists, _ := cli.BucketExists(ctx, "mybucket")
if !exists {
    err := cli.MakeBucket(ctx, "mybucket", minio.MakeBucketOptions{Region: "cn-north-1"})
}
```

#### 流式上传与下载(io.Reader 语义)

```go
// 流式上传:任意 io.Reader,如 bytes.Buffer、multipart 文件、网络流
_, err := cli.PutObject(ctx, "mybucket", "logs/app.log",
    bytes.NewReader(data), int64(len(data)),
    minio.PutObjectOptions{ContentType: "text/plain"})

// 流式下载:GetObject 返回 *minio.Object,实现 io.ReadCloser,支持 Range 读
obj, _ := cli.GetObject(ctx, "mybucket", "logs/app.log", minio.GetObjectOptions{})
```

#### 文件直传直下

```go
// 本地文件直接上传/下载到磁盘,内部自动处理大文件分片
_, err := cli.FPutObject(ctx, "mybucket", "photos/a.jpg", "/tmp/a.jpg", minio.PutObjectOptions{})
_, err = cli.FGetObject(ctx, "mybucket", "photos/a.jpg", "/tmp/b.jpg", minio.GetObjectOptions{})
```

#### 预签名 URL

```go
// 生成临时下载链接,可直接给浏览器或第三方使用,无需暴露凭证
u, err := cli.PresignedGetObject(ctx, "mybucket", "photos/a.jpg", 15*time.Minute, url.Values{})
// *u 形如 https://host/mybucket/photos/a.jpg?X-Amz-...
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 后 `minio.New` 创建 client,DSN 带 bucket 时执行 `BucketExists` 探活
- `Shutdown`:minio client 内部管理连接,无需显式关闭

```go
cli, err := f.Get().Client(ctx)
if errors.Is(err, s3.ErrNotEnabled) {
    // s3 组件未启用,走降级逻辑
}
```

常见错误哨兵:`ErrNotEnabled`/`ErrNotReady`(组件未启用/未就绪)、`ErrInvalidDSN`、`ErrInvalidScheme`、`ErrBucketNotFound`(DSN 声明的 bucket 不存在)。

## elasticsearch

Weaver 的 Elasticsearch 客户端组件,基于官方 [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch)。声明 `weaver.Ref[elasticsearch.T]` 即可注入 `*elasticsearch.Client`,节点配置与 Ping 探活由框架生命周期托管。

### 安装

```bash
go get github.com/jun3372/weaver/components/elasticsearch
```

```go
import _ "github.com/jun3372/weaver/components/elasticsearch" // 匿名导入完成组件注册
```

### 配置

```yaml
elasticsearch:
  Enabled: true
  DSN: "http://localhost:9200"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `http://localhost:9200` | 节点地址,多节点逗号分隔;userinfo 携带 Basic Auth,如 `http://elastic:secret@host:9200`(取首个带 userinfo 的地址) |

环境变量覆盖:`WEAVER_ELASTICSEARCH_DSN`、`WEAVER_ELASTICSEARCH_ENABLED`。

### 基础用法

```go
type searchService struct {
    weaver.Implements[SearchService]
    weaver.Ref[elasticsearch.T]
}

func (s *searchService) Search(ctx context.Context, q string) ([]byte, error) {
    cli, err := s.Get().Client(ctx)
    if err != nil {
        return nil, err
    }
    res, err := cli.Search(cli.Search.WithContext(ctx), cli.Search.WithQuery(q))
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()
    if res.IsError() {
        return nil, errors.Errorf("search 失败: %s", res.Status())
    }
    return io.ReadAll(res.Body)
}
```

### 进阶场景

#### 写入文档

```go
cli, _ := s.Get().Client(ctx)

// esutil.NewJSONReader 把任意值序列化成请求体
res, err := cli.Index(
    "articles",                        // 索引名
    esutil.NewJSONReader(map[string]any{"title": "hello", "views": 0}),
    cli.Index.WithContext(ctx),
    cli.Index.WithDocumentID("1"),     // 不指定则自动生成
    cli.Index.WithRefresh("false"),
)
defer res.Body.Close()
```

#### Search(JSON query body)

```go
query := `{
  "query": {
    "bool": {
      "must": [{"match": {"title": "golang"}}],
      "filter": [{"range": {"views": {"gte": 100}}}]
    }
  }
}`
res, err := cli.Search(
    cli.Search.WithContext(ctx),
    cli.Search.WithIndex("articles"),
    cli.Search.WithBody(strings.NewReader(query)),
)
defer res.Body.Close()
```

#### BulkIndexer 批量写入

```go
bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
    Index:         "articles", // 也可在 item 上单独指定
    Client:        cli,
    NumWorkers:    4,
    FlushInterval: 30 * time.Second,
})
if err != nil {
    return err
}

for _, doc := range docs {
    err = bi.Add(ctx, esutil.BulkIndexerItem{
        Action:     "index",
        DocumentID: doc.ID,
        Body:       bytes.NewReader(doc.JSON),
        OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem,
            resp esutil.BulkIndexerResponseItem, err error) {
            // 单条失败回调,如记录日志
        },
    })
    if err != nil {
        return err
    }
}
return bi.Close(ctx) // 等待全部刷盘并关闭
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 得到地址列表后创建 client,并 `Ping` 探活(响应为错误状态同样判失败)
- `Shutdown`:官方 client 内部管理连接,无需显式关闭
- 常见错误:`ErrNotEnabled`/`ErrNotReady`(组件未启用/未就绪);响应级别失败须通过 `res.IsError()` 判断并记得 `res.Body.Close()`

## meilisearch

Weaver 的 Meilisearch 客户端组件,基于官方 [meilisearch-go](https://github.com/meilisearch/meilisearch-go)。声明 `weaver.Ref[meilisearch.T]` 即可注入 `meilisearch.ServiceManager`,节点配置与 Health 探活由框架生命周期托管。Meilisearch 是开箱即用的搜索引擎,无需预先定义 mapping,适合站内搜索、电商筛选等场景。

### 安装

```bash
go get github.com/jun3372/weaver/components/meilisearch
```

```go
import _ "github.com/jun3372/weaver/components/meilisearch" // 匿名导入完成组件注册
```

### 配置

```yaml
meilisearch:
  Enabled: true
  Host: "http://localhost:7700"
  APIKey: "masterKey"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `Host` | `http://localhost:7700` | 节点地址,仅接受 `http://` / `https://` 形式(Meilisearch 单节点,无多地址) |
| `APIKey` | 空 | API Key(Bearer 认证),留空则匿名访问 |

环境变量覆盖:`WEAVER_MEILISEARCH_HOST`、`WEAVER_MEILISEARCH_APIKEY`、`WEAVER_MEILISEARCH_ENABLED`。

### 基础用法

```go
type searchService struct {
    weaver.Implements[SearchService]
    weaver.Ref[meilisearch.T]
}

func (s *searchService) Search(ctx context.Context, index, q string) ([]interface{}, error) {
    cli, err := s.Get().Client(ctx)
    if err != nil {
        return nil, err
    }
    res, err := cli.Index(index).SearchWithContext(ctx, q, &meilisearch.SearchRequest{})
    if err != nil {
        return nil, err
    }
    return res.Hits, nil
}
```

### 进阶场景

#### 写入文档

```go
cli, _ := s.Get().Client(ctx)

// 写入是异步任务,返回 TaskInfo;可用 cli.WaitForTask 等待完成
task, err := cli.Index("articles").AddDocumentsWithContext(ctx, []map[string]any{
    {"id": 1, "title": "hello weaver", "views": 0},
}, nil)
if err != nil {
    return err
}
_, err = cli.Index("articles").WaitForTaskWithContext(ctx, task.TaskUID)
```

#### 过滤与分页

```go
res, err := cli.Index("articles").SearchWithContext(ctx, "golang", &meilisearch.SearchRequest{
    Filter:               "views >= 100",
    Limit:                20,
    AttributesToRetrieve: []string{"id", "title"},
})
```

> 过滤字段需先在索引设置中声明为 filterableAttributes,如
> `cli.Index("articles").UpdateFilterableAttributesWithContext(ctx, &[]interface{}{"views"})`。

### 生命周期与错误

- `Init`:未启用时打日志直接返回;校验 `Host` 后 `meilisearch.New` 创建 client,并 `HealthWithContext` 探活(状态非 `available` 同样判失败)
- `Shutdown`:v0.36 的 `ServiceManager` 内部管理连接,未暴露关闭接口,无需显式关闭
- 常见错误:`ErrNotEnabled`/`ErrNotReady`(组件未启用/未就绪)、`ErrInvalidHost`(Host 缺少合法 scheme)

## 相关阅读

- [组件总览](/guide/components)
- [配置管理](/guide/config)
