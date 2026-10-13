# weaver plugins/meilisearch

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/plugins/meilisearch.svg)](https://pkg.go.dev/github.com/jun3372/weaver/plugins/meilisearch)

Weaver 的 Meilisearch 客户端组件，基于官方 [meilisearch-go](https://github.com/meilisearch/meilisearch-go)。声明 `weaver.Ref[meilisearch.T]` 即可注入，节点配置、探活由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/plugins/meilisearch
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/plugins/meilisearch"
```

## 配置

```yaml
meilisearch:
  Enabled: true
  Host: "http://localhost:7700"
  APIKey: "masterKey"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `Host` | `http://localhost:7700` | 节点地址，仅接受 `http://` / `https://` 形式（Meilisearch 单节点，无多地址） |
| `APIKey` | 空 | API Key（Bearer 认证），留空则匿名访问 |

环境变量覆盖：`WEAVER_MEILISEARCH_HOST`、`WEAVER_MEILISEARCH_APIKEY`、`WEAVER_MEILISEARCH_ENABLED`。

## 使用示例

```go
type searchService struct {
    weaver.Implements[SearchService]
    weaver.Ref[meilisearch.T]
}

func (s *searchService) Search(ctx context.Context, index, q string) ([]interface{}, error) {
    cli, err := s.Ref().Client(ctx)
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

## 生命周期

- `Init`：未启用时打日志直接返回；校验 `Host` 后创建 client，并 `HealthWithContext` 探活（状态非 `available` 同样判失败）
- `Shutdown`：v0.36 的 `ServiceManager` 内部管理连接，未暴露关闭接口，无需显式关闭

## 完整教程

更多用法见文档站的[存储与搜索教程](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/storage.md#meilisearch)。
