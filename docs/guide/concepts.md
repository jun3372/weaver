# 核心概念：组件与依赖注入

Weaver 的核心是基于接口的组件系统。每个组件由**接口**定义、**实现结构体**承载，框架在启动时根据生成的注册代码自动构建组件图，并按依赖关系实例化。

## 定义组件接口

```go
package user

import "context"

type User interface {
    SayHello(ctx context.Context, name string) (string, error)
}
```

## 实现组件

通过嵌入 `weaver.Implements[T]` 声明"这个结构体实现了组件接口 T"：

```go
package user

import (
    "context"
    "fmt"

    "github.com/jun3372/weaver"
)

type option struct {
    Source string
    Type   string
}

type userImpl struct {
    weaver.Implements[User]
    weaver.WithConfig[option] `conf:"user"`
}

func (u *userImpl) Init(ctx context.Context) error {
    u.Logger(ctx).Info("User component initialized")
    return nil
}

func (u *userImpl) SayHello(ctx context.Context, name string) (string, error) {
    return fmt.Sprintf("Hello, %s!", name), nil
}
```

## 注入依赖

在需要使用其他组件的地方声明 `weaver.Ref[T]` 字段，框架会自动注入实例：

```go
type app struct {
    weaver.Implements[weaver.Main]
    weaver.WithConfig[options] `conf:"app"`
    user weaver.Ref[user.User] // 引用 User 组件
}

func (a *app) Init(ctx context.Context) error {
    userComponent := a.user.Get()

    greeting, err := userComponent.SayHello(ctx, "World")
    if err != nil {
        return err
    }

    a.Logger(ctx).Info(greeting)
    return nil
}
```

`Ref` 也支持匿名嵌入（如 `weaver.Ref[user.User]` 直接作为结构体字段），同样会被自动注入。

## 工作原理

- `weaver generate` 扫描包中嵌入了 `weaver.Implements[T]` 的结构体，生成 `weaver_gen.go`，其中包含所有组件的注册信息。
- `weaver.Run` 启动时根据注册信息构建三张索引：按组件名、按接口类型、按实现类型。
- 组件实例通过反射创建，注入顺序为：退出函数 → 日志器 → 配置（`WithConfig`）→ 依赖（`Ref`），最后调用 `Init`。
- 组件按需懒加载：只有被引用到的组件才会被实例化。

## 常见错误

如果看到类似下面的错误，说明注册代码缺失或过期：

```text
component github.com/xxx.User not found; maybe you forgot to run weaver generate
```

重新执行 `weaver generate .` 即可。详见[代码生成工具](/guide/generate)。
