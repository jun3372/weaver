# weaver components/elasticsearch

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/elasticsearch.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/elasticsearch)

Weaver 的 Elasticsearch 客户端组件，基于官方 [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch)。声明 `weaver.Ref[elasticsearch.T]` 即可注入，节点配置、探活由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/elasticsearch
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/elasticsearch"
```

## 配置

```yaml
elasticsearch:
  Enabled: true
  DSN: "http://localhost:9200"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `http://localhost:9200` | 节点地址，多节点逗号分隔；支持 userinfo 携带 Basic Auth，如 `http://elastic:secret@host:9200`（取首个带 userinfo 的地址） |

环境变量覆盖：`WEAVER_ELASTICSEARCH_DSN`、`WEAVER_ELASTICSEARCH_ENABLED`。

## 使用示例

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
    return io.ReadAll(res.Body)
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；解析 DSN 得到地址列表后创建 client，并 `Ping` 探活（响应为错误状态同样判失败）
- `Shutdown`：官方 client 内部管理连接，无需显式关闭

## 完整教程

更多用法见文档站的[存储与搜索教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/storage.md#elasticsearch)。
