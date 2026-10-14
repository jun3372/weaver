# Storage & Search: s3 / elasticsearch / meilisearch

## s3

Weaver's S3-compatible object storage client component, built on [minio-go/v7](https://github.com/minio/minio-go). A single component covers MinIO, AWS S3 and every other S3-compatible store. Declare a `weaver.Ref[s3.T]` to inject a `*minio.Client`; client initialization and the bucket health check are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/components/s3
```

```go
import _ "github.com/jun3372/weaver/components/s3" // blank import completes component registration
```

### Configuration

```yaml
s3:
  Enabled: true
  DSN: "s3://minioadmin:minioadmin@localhost:9000?secure=false"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the component is enabled; when disabled, injection calls return `ErrNotEnabled` |
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

### Advanced Scenarios

#### Creating a Bucket

```go
cli, _ := f.Get().Client(ctx)

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
cli, err := f.Get().Client(ctx)
if errors.Is(err, s3.ErrNotEnabled) {
    // s3 component not enabled; fall back to degraded logic
}
```

Common error sentinels: `ErrNotEnabled`/`ErrNotReady` (component not enabled / not ready), `ErrInvalidDSN`, `ErrInvalidScheme`, `ErrBucketNotFound` (the bucket declared in the DSN does not exist).

## elasticsearch

Weaver's Elasticsearch client component, built on the official [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch). Declare a `weaver.Ref[elasticsearch.T]` to inject an `*elasticsearch.Client`; node configuration and the `Ping` health check are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/components/elasticsearch
```

```go
import _ "github.com/jun3372/weaver/components/elasticsearch" // blank import completes component registration
```

### Configuration

```yaml
elasticsearch:
  Enabled: true
  DSN: "http://localhost:9200"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the component is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `http://localhost:9200` | Node addresses, comma-separated for multiple nodes; userinfo carries Basic Auth, e.g. `http://elastic:secret@host:9200` (the first address with userinfo is used) |

Environment variable overrides: `WEAVER_ELASTICSEARCH_DSN`, `WEAVER_ELASTICSEARCH_ENABLED`.

### Basic Usage

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
        return nil, errors.Errorf("search failed: %s", res.Status())
    }
    return io.ReadAll(res.Body)
}
```

### Advanced Scenarios

#### Indexing Documents

```go
cli, _ := s.Get().Client(ctx)

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
- Common errors: `ErrNotEnabled`/`ErrNotReady` (component not enabled / not ready); response-level failures must be detected via `res.IsError()`, and remember to call `res.Body.Close()`

## meilisearch

Weaver's Meilisearch client component, built on the official [meilisearch-go](https://github.com/meilisearch/meilisearch-go). Declare a `weaver.Ref[meilisearch.T]` to inject a `meilisearch.ServiceManager`; node configuration and the `Health` probe are managed by the framework lifecycle. Meilisearch is a batteries-included search engine with no mapping to define up front — a good fit for site search and e-commerce filtering.

### Installation

```bash
go get github.com/jun3372/weaver/components/meilisearch
```

```go
import _ "github.com/jun3372/weaver/components/meilisearch" // blank import completes component registration
```

### Configuration

```yaml
meilisearch:
  Enabled: true
  Host: "http://localhost:7700"
  APIKey: "masterKey"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the component is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `Host` | `http://localhost:7700` | Node address, only `http://` / `https://` forms are accepted (Meilisearch is single-node, no multi-address) |
| `APIKey` | empty | API Key (Bearer auth); leave empty for anonymous access |

Environment variable overrides: `WEAVER_MEILISEARCH_HOST`, `WEAVER_MEILISEARCH_APIKEY`, `WEAVER_MEILISEARCH_ENABLED`.

### Basic Usage

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

### Advanced Scenarios

#### Indexing Documents

```go
cli, _ := s.Get().Client(ctx)

// Writes are async tasks returning TaskInfo; use cli.WaitForTask to await completion
task, err := cli.Index("articles").AddDocumentsWithContext(ctx, []map[string]any{
    {"id": 1, "title": "hello weaver", "views": 0},
}, nil)
if err != nil {
    return err
}
_, err = cli.Index("articles").WaitForTaskWithContext(ctx, task.TaskUID)
```

#### Filtering and Pagination

```go
res, err := cli.Index("articles").SearchWithContext(ctx, "golang", &meilisearch.SearchRequest{
    Filter:               "views >= 100",
    Limit:                20,
    AttributesToRetrieve: []string{"id", "title"},
})
```

> Filterable fields must be declared in the index settings first, e.g.
> `cli.Index("articles").UpdateFilterableAttributesWithContext(ctx, &[]interface{}{"views"})`.

### Lifecycle & Errors

- `Init`: if not enabled, logs and returns immediately; validates `Host`, creates the client with `meilisearch.New` and probes with `HealthWithContext` (a status other than `available` also counts as failure)
- `Shutdown`: the v0.36 `ServiceManager` manages connections internally and exposes no close method; no explicit close needed
- Common errors: `ErrNotEnabled`/`ErrNotReady` (component not enabled / not ready), `ErrInvalidHost` (Host missing a valid scheme)

## Further Reading

- [Components Overview](/en/guide/components)
- [Configuration](/en/guide/config)
