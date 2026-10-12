# Storage & Search: s3 / elasticsearch

## s3

Weaver's S3-compatible object storage client component, built on [minio-go/v7](https://github.com/minio/minio-go). A single plugin covers MinIO, AWS S3 and every other S3-compatible store. Declare a `weaver.Ref[s3.T]` to inject a `*minio.Client`; client initialization and the bucket health check are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/s3
```

```go
import _ "github.com/jun3372/weaver/plugins/s3" // blank import completes component registration
```

### Configuration

```yaml
s3:
  Enabled: true
  DSN: "s3://minioadmin:minioadmin@localhost:9000?secure=false"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `s3://minioadmin:minioadmin@localhost:9000?secure=false` | Format `s3://access:secret@host:port[/bucket]?secure=true\|false` |

DSN details: `bucket` is optional; when set, the `Init` stage runs a `BucketExists` probe and reports `ErrBucketNotFound` if it does not exist. If `secure` is not specified it defaults to `false` (the local MinIO case; when connecting to AWS S3 remember to set `secure=true`).

Environment variable overrides: `WEAVER_S3_DSN`, `WEAVER_S3_ENABLED`.

### Basic Usage

```go
type fileStore struct {
    weaver.Implements[FileStore]
    weaver.Ref[s3.T]
}

func (f *fileStore) Upload(ctx context.Context, object, path string) error {
    cli, err := f.Ref().Client(ctx)
    if err != nil {
        return err
    }
    _, err = cli.FPutObject(ctx, "mybucket", object, path, minio.PutObjectOptions{})
    return err
}

func (f *fileStore) Download(ctx context.Context, object string) ([]byte, error) {
    cli, err := f.Ref().Client(ctx)
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

### Advanced Scenarios

#### Creating a Bucket

```go
cli, _ := f.Ref().Client(ctx)

exists, _ := cli.BucketExists(ctx, "mybucket")
if !exists {
    err := cli.MakeBucket(ctx, "mybucket", minio.MakeBucketOptions{Region: "cn-north-1"})
}
```

#### Streaming Upload and Download (io.Reader Semantics)

```go
// Streaming upload: any io.Reader, e.g. bytes.Buffer, a multipart file, a network stream
_, err := cli.PutObject(ctx, "mybucket", "logs/app.log",
    bytes.NewReader(data), int64(len(data)),
    minio.PutObjectOptions{ContentType: "text/plain"})

// Streaming download: GetObject returns a *minio.Object implementing io.ReadCloser, with Range read support
obj, _ := cli.GetObject(ctx, "mybucket", "logs/app.log", minio.GetObjectOptions{})
```

#### Direct File Upload and Download

```go
// Upload/download local files straight to/from disk; large-file multipart handling is automatic
_, err := cli.FPutObject(ctx, "mybucket", "photos/a.jpg", "/tmp/a.jpg", minio.PutObjectOptions{})
_, err = cli.FGetObject(ctx, "mybucket", "photos/a.jpg", "/tmp/b.jpg", minio.GetObjectOptions{})
```

#### Presigned URLs

```go
// Generate a temporary download link for browsers or third parties without exposing credentials
u, err := cli.PresignedGetObject(ctx, "mybucket", "photos/a.jpg", 15*time.Minute, url.Values{})
// *u looks like https://host/mybucket/photos/a.jpg?X-Amz-...
```

### Lifecycle & Errors

- `Init`: if not enabled, logs and returns immediately; parses the DSN and creates the client with `minio.New`; when the DSN carries a bucket, runs a `BucketExists` probe
- `Shutdown`: the minio client manages connections internally; no explicit close needed

```go
cli, err := f.Ref().Client(ctx)
if errors.Is(err, s3.ErrNotEnabled) {
    // s3 plugin not enabled; fall back to degraded logic
}
```

Common error sentinels: `ErrNotEnabled`/`ErrNotReady` (plugin not enabled / not ready), `ErrInvalidDSN`, `ErrInvalidScheme`, `ErrBucketNotFound` (the bucket declared in the DSN does not exist).

## elasticsearch

Weaver's Elasticsearch client component, built on the official [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch). Declare a `weaver.Ref[elasticsearch.T]` to inject an `*elasticsearch.Client`; node configuration and the `Ping` health check are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/elasticsearch
```

```go
import _ "github.com/jun3372/weaver/plugins/elasticsearch" // blank import completes component registration
```

### Configuration

```yaml
elasticsearch:
  Enabled: true
  DSN: "http://localhost:9200"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `http://localhost:9200` | Node addresses, comma-separated for multiple nodes; userinfo carries Basic Auth, e.g. `http://elastic:secret@host:9200` (the first address with userinfo is used) |

Environment variable overrides: `WEAVER_ELASTICSEARCH_DSN`, `WEAVER_ELASTICSEARCH_ENABLED`.

### Basic Usage

```go
type searchService struct {
    weaver.Implements[SearchService]
    weaver.Ref[elasticsearch.T]
}

func (s *searchService) Search(ctx context.Context, q string) ([]byte, error) {
    cli, err := s.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    res, err := cli.Search(cli.Search.WithContext(ctx), cli.Search.WithQuery(q))
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()
    if res.IsError() {
        return nil, errors.Errorf("search failed: %s", res.Status())
    }
    return io.ReadAll(res.Body)
}
```

### Advanced Scenarios

#### Indexing Documents

```go
cli, _ := s.Ref().Client(ctx)

// esutil.NewJSONReader serializes any value into a request body
res, err := cli.Index(
    "articles",                        // index name
    esutil.NewJSONReader(map[string]any{"title": "hello", "views": 0}),
    cli.Index.WithContext(ctx),
    cli.Index.WithDocumentID("1"),     // auto-generated if not specified
    cli.Index.WithRefresh("false"),
)
defer res.Body.Close()
```

#### Search (JSON query body)

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

#### BulkIndexer Batch Writes

```go
bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
    Index:         "articles", // can also be specified per item
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
            // per-item failure callback, e.g. log the error
        },
    })
    if err != nil {
        return err
    }
}
return bi.Close(ctx) // wait for everything to flush and close
```

### Lifecycle & Errors

- `Init`: if not enabled, logs and returns immediately; parses the DSN into an address list, creates the client and probes with `Ping` (an error-level response also counts as failure)
- `Shutdown`: the official client manages connections internally; no explicit close needed
- Common errors: `ErrNotEnabled`/`ErrNotReady` (plugin not enabled / not ready); response-level failures must be detected via `res.IsError()`, and remember to call `res.Body.Close()`

## Further Reading

- [Plugins Overview](/en/guide/plugins)
- [Configuration](/en/guide/config)
