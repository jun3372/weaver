# weaver plugins/mqtt

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/plugins/mqtt.svg)](https://pkg.go.dev/github.com/jun3372/weaver/plugins/mqtt)

Weaver 的 MQTT 客户端组件，基于 [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang)，适用于 IoT 消息场景。声明 `weaver.Ref[mqtt.T]` 即可注入，连接建立与优雅断开由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/plugins/mqtt
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/plugins/mqtt"
```

## 配置

```yaml
mqtt:
  Enabled: true
  DSN: "mqtt://localhost:1883"
  ClientID: "weaver-app"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `mqtt://localhost:1883` | broker 地址，scheme 支持 `mqtt` / `tcp` / `ssl` / `tls` / `ws` / `wss`；userinfo 转 broker 认证 |
| `ClientID` | `weaver` | MQTT ClientID，同 broker 集群内需唯一 |

环境变量覆盖：`WEAVER_MQTT_DSN`、`WEAVER_MQTT_CLIENTID`、`WEAVER_MQTT_ENABLED`。

## 使用示例

```go
type deviceGateway struct {
    weaver.Implements[DeviceGateway]
    weaver.Ref[mqtt.T]
}

func (g *deviceGateway) Publish(ctx context.Context, topic string, payload []byte) error {
    cli, err := g.Ref().Client(ctx)
    if err != nil {
        return err
    }
    token := cli.Publish(topic, 1, false, payload)
    token.Wait()
    return token.Error()
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；校验 DSN scheme 后建连（5 秒超时），失败自动 `Disconnect` 并返回错误
- `Shutdown`：`Disconnect(250ms)`，给在途消息留冲刷时间

## 完整教程

更多用法见文档站的[消息队列教程](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/messaging.md#mqtt)。
