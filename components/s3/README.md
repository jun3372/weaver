# weaver components/s3

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/s3.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/s3)

Weaver 的 S3 兼容对象存储客户端组件，基于 [minio-go/v7](https://github.com/minio/minio-go)。一个组件同时覆盖 MinIO、AWS S3 及其他一切 S3 兼容存储。声明 `weaver.Ref[s3.T]` 即可注入，client 初始化与 bucket 探活由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/s3
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/s3"
```

## 配置

```yaml
s3:
  Enabled: true
  DSN: "s3://minioadmin:minioadmin@localhost:9000?secure=false"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `s3://minioadmin:minioadmin@localhost:9000?secure=false` | 见下方格式说明 |

### DSN 格式

```
s3://<accessKey>:<secretKey>@<endpoint>[/<bucket>][?secure=true|false]
```

- `accessKey` / `secretKey`：静态凭证（必填）
- `endpoint`：服务地址，如 `localhost:9000` 或 `s3.amazonaws.com`（必填）
- `bucket`：可选。配置后 `Init` 阶段执行 `BucketExists` 探活，bucket 不存在返回 `ErrBucketNotFound`
- `secure`：是否走 HTTPS，未指定默认 `false`（本地 MinIO 场景；AWS S3 记得写 `secure=true`）

环境变量覆盖：`WEAVER_S3_DSN`、`WEAVER_S3_ENABLED`。

## 使用示例

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
```

## 生命周期

- `Init`：未启用时打日志直接返回；解析 DSN 后 `minio.New` 创建 client，bucket 已配置时执行 `BucketExists` 探活
- `Shutdown`：minio client 无需显式关闭

## 完整教程

更多用法见文档站的[存储与搜索教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/storage.md#s3)。
