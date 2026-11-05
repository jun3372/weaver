---
layout: home

nav:
  - text: 指南
    link: /guide/getting-started
  - text: 组件
    link: /guide/components
  - text: 智能体规范
    link: /LLM
  - text: 示例
    link: /examples

altLang:
  text: English
  link: /en/

hero:
  badge: v0.1.11 · MIT License · Go 1.27
  title: 轻量级 Go 组件化应用框架
  tagline: 依赖注入 · 配置管理 · 生命周期 · 结构化日志 · 分布式追踪
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: GitHub
      link: https://github.com/jun3372/weaver
      external: true
  terminal:
    title: quickstart
    lines:
      - { t: cmd, s: weaver init hello }
      - { t: ok, s: '✔ 项目骨架已生成 ./hello' }
      - { t: cmd, s: 'cd hello && weaver generate ./' }
      - { t: ok, s: '✔ 组件注册代码已写入 weaver_gen.go' }
      - { t: cmd, s: 'go run .' }
      - { t: info, s: 'INFO App init' }
      - { t: info, s: 'INFO resp msg="Hello, jun3372!"' }
      - { t: out, s: '# Ctrl+C → 按依赖反向拓扑优雅退出' }

stepsTitle: 三步上手
steps:
  - title: 声明组件
    details: 用接口定义组件，以 weaver.Implements 声明实现；依赖通过 weaver.Ref 自动注入。
    lang: go
    code: |-
      type Greeter interface {
          Greet(ctx context.Context, name string) (string, error)
      }

      type greeter struct {
          weaver.Implements[Greeter]
      }
  - title: 生成注册代码
    details: weaver generate 扫描组件接口，把注册代码写入 weaver_gen.go，无需手写装配。
    lang: sh
    code: |-
      $ weaver generate ./
      ✔ weaver_gen.go 已生成
  - title: 运行
    details: weaver.Run 按依赖图并发启动组件，Init / Start / Shutdown 钩子托管全生命周期。
    lang: sh
    code: |-
      $ go run .
      INFO App init
      INFO resp msg="Hello, jun3372!"

featuresTitle: 核心能力
features:
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14.086 18.412A2 2 0 0112.67 19H5v-7.672a2 2 0 01.586-1.414L11.75 3.75a6 6 0 118.49 8.49z" /><path d="M16 8 2 22" /><path d="M17.488 15H9" /></svg>
    title: 轻量级设计
    details: 核心代码简洁高效，基于 Go 标准库与少量精选依赖，无框架强绑定。
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M2.97 12.92A2 2 0 0 0 2 14.63v3.24a2 2 0 0 0 .97 1.71l3 1.8a2 2 0 0 0 2.06 0L12 19v-5.5l-5-3-4.03 2.42Z" /><path d="m7 16.5-4.74-2.85" /><path d="m7 16.5 5-3" /><path d="M7 16.5v5.17" /><path d="M12 13.5V19l3.97 2.38a2 2 0 0 0 2.06 0l3-1.8a2 2 0 0 0 .97-1.71v-3.24a2 2 0 0 0-.97-1.71L17 10.5l-5 3Z" /><path d="m17 16.5-5-3" /><path d="m17 16.5 4.74-2.85" /><path d="M17 16.5v5.17" /><path d="M7.97 4.42A2 2 0 0 0 7 6.13v4.37l5 3 5-3V6.13a2 2 0 0 0-.97-1.71l-3-1.8a2 2 0 0 0-2.06 0l-3 1.8Z" /><path d="M12 8 7.26 5.15" /><path d="m12 8 4.74-2.85" /><path d="M12 13.5V8" /></svg>
    title: 组件化架构
    details: 基于接口的组件系统，通过 weaver.Implements 声明组件，weaver generate 自动生成注册代码。
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15 6a9 9 0 0 0-9 9V3" /><circle cx="18" cy="6" r="3" /><circle cx="6" cy="18" r="3" /></svg>
    title: 依赖注入
    details: 通过 weaver.Ref 声明组件依赖，框架自动构建组件图并按需实例化。
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 17H5" /><path d="M19 7h-9" /><circle cx="17" cy="17" r="3" /><circle cx="7" cy="7" r="3" /></svg>
    title: 灵活的配置管理
    details: 基于 Viper，支持 YAML / TOML / JSON 等多种格式，WithConfig 泛型注入，配置变更自动生效。
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6 22a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.704.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2z" /><path d="M14 2v5a1 1 0 0 0 1 1h5" /><path d="M10 9H8" /><path d="M16 13H8" /><path d="M16 17H8" /></svg>
    title: 结构化日志
    details: 基于 Go 标准库 slog，自动携带 TraceID / SpanID，支持轮转压缩的文件输出。
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2" /></svg>
    title: OpenTelemetry 集成
    details: 开箱即用的分布式追踪，日志与链路自动关联。

examples:
  title: 示例项目
  details: 从最小 Hello 到标准 DDD 分层，全部开箱即可运行。
  items:
    - name: examples/hello
      tag: 入门
      title: 最小示例
      details: 组件声明、依赖注入、配置注入与结构化日志的最小可运行组合。
      link: /examples
    - name: examples/http
      tag: 服务托管
      title: HTTP / TCP / UDP 三服务
      details: Listener 服务组件、路由前缀挂载与配置热更新实战。
      link: /examples
    - name: examples/ddd
      tag: 架构
      title: 标准 DDD 分层
      details: 领域端口、同包仓储组件、GORM SQLite 与统一 HTTP 响应信封。
      link: /examples

footer:
  message: Released under the MIT License.
  copyright: Copyright © 2026 jun3372
---
