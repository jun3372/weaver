---
layout: home

nav:
  - text: Guide
    link: /en/guide/getting-started
  - text: Components
    link: /en/guide/components
  - text: Agent Guide
    link: /LLM
  - text: Examples
    link: /en/examples

altLang:
  text: 简体中文
  link: /

hero:
  badge: v0.1.11 · MIT License · Go 1.27
  title: Lightweight Component-Based Go Framework
  tagline: Dependency Injection · Configuration · Lifecycle · Structured Logging · Distributed Tracing
  actions:
    - theme: brand
      text: Get Started
      link: /en/guide/getting-started
    - theme: alt
      text: GitHub
      link: https://github.com/jun3372/weaver
      external: true
  terminal:
    title: quickstart
    lines:
      - { t: cmd, s: weaver init hello }
      - { t: ok, s: '✔ project scaffold created ./hello' }
      - { t: cmd, s: 'cd hello && weaver generate ./' }
      - { t: ok, s: '✔ registration code written to weaver_gen.go' }
      - { t: cmd, s: 'go run .' }
      - { t: info, s: 'INFO App init' }
      - { t: info, s: 'INFO resp msg="Hello, jun3372!"' }
      - { t: out, s: '# Ctrl+C → graceful shutdown in reverse dependency order' }

stepsTitle: Three Steps to Run
steps:
  - title: Declare a Component
    details: Define components as interfaces and declare implementations with weaver.Implements; dependencies are injected via weaver.Ref.
    lang: go
    code: |-
      type Greeter interface {
          Greet(ctx context.Context, name string) (string, error)
      }

      type greeter struct {
          weaver.Implements[Greeter]
      }
  - title: Generate Registration
    details: weaver generate scans component interfaces and writes registration code to weaver_gen.go — no hand wiring.
    lang: sh
    code: |-
      $ weaver generate ./
      ✔ weaver_gen.go generated
  - title: Run
    details: weaver.Run starts components concurrently by the dependency graph; Init / Start / Shutdown hooks own the lifecycle.
    lang: sh
    code: |-
      $ go run .
      INFO App init
      INFO resp msg="Hello, jun3372!"

featuresTitle: Core Capabilities
features:
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14.086 18.412A2 2 0 0112.67 19H5v-7.672a2 2 0 01.586-1.414L11.75 3.75a6 6 0 118.49 8.49z" /><path d="M16 8 2 22" /><path d="M17.488 15H9" /></svg>
    title: Lightweight
    details: A concise core built on the Go standard library and a small set of hand-picked dependencies. No framework lock-in.
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M2.97 12.92A2 2 0 0 0 2 14.63v3.24a2 2 0 0 0 .97 1.71l3 1.8a2 2 0 0 0 2.06 0L12 19v-5.5l-5-3-4.03 2.42Z" /><path d="m7 16.5-4.74-2.85" /><path d="m7 16.5 5-3" /><path d="M7 16.5v5.17" /><path d="M12 13.5V19l3.97 2.38a2 2 0 0 0 2.06 0l3-1.8a2 2 0 0 0 .97-1.71v-3.24a2 2 0 0 0-.97-1.71L17 10.5l-5 3Z" /><path d="m17 16.5-5-3" /><path d="m17 16.5 4.74-2.85" /><path d="M17 16.5v5.17" /><path d="M7.97 4.42A2 2 0 0 0 7 6.13v4.37l5 3 5-3V6.13a2 2 0 0 0-.97-1.71l-3-1.8a2 2 0 0 0-2.06 0l-3 1.8Z" /><path d="M12 8 7.26 5.15" /><path d="m12 8 4.74-2.85" /><path d="M12 13.5V8" /></svg>
    title: Component Architecture
    details: Interface-based component system. Declare components with weaver.Implements and let weaver generate wire up registration code.
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15 6a9 9 0 0 0-9 9V3" /><circle cx="18" cy="6" r="3" /><circle cx="6" cy="18" r="3" /></svg>
    title: Dependency Injection
    details: Declare dependencies with weaver.Ref and the framework builds the component graph, instantiating on demand.
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 17H5" /><path d="M19 7h-9" /><circle cx="17" cy="17" r="3" /><circle cx="7" cy="7" r="3" /></svg>
    title: Flexible Configuration
    details: Powered by Viper with YAML / TOML / JSON support, generic injection via WithConfig, and hot reload.
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6 22a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.704.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2z" /><path d="M14 2v5a1 1 0 0 0 1 1h5" /><path d="M10 9H8" /><path d="M16 13H8" /><path d="M16 17H8" /></svg>
    title: Structured Logging
    details: Built on the standard library slog, with automatic TraceID / SpanID and rotating, compressed file output.
  - icon: >-
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2" /></svg>
    title: OpenTelemetry Ready
    details: First-class distributed tracing with logs automatically correlated to traces.

examples:
  title: Example Projects
  details: From a minimal Hello to a standard DDD layering — all runnable out of the box.
  items:
    - name: examples/hello
      tag: Starter
      title: Minimal Example
      details: The smallest runnable combination of components, DI, config injection, and structured logging.
      link: /en/examples
    - name: examples/http
      tag: Server Hosting
      title: HTTP / TCP / UDP Servers
      details: Listener service components, route prefix mounting, and config hot reload in practice.
      link: /en/examples
    - name: examples/ddd
      tag: Architecture
      title: Standard DDD Layering
      details: Domain ports, same-package repository components, GORM SQLite, and a unified HTTP response envelope.
      link: /en/examples

footer:
  message: Released under the MIT License.
  copyright: Copyright © 2026 jun3372
---
