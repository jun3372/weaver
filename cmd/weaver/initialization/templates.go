package initialization

import (
	"os/exec"
	"text/template"
)

func execGo(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var mainTmpl = template.Must(template.New("main").Parse(`package main

import (
	"context"

	"{{.Module}}/internal/app"

	"github.com/jun3372/weaver"
)

type option struct {
	Name    string
	Version string
}

// Main 是应用主组件:声明依赖(HTTP 服务)与配置,由 weaver.Run 装配启动。
type Main struct {
	weaver.Implements[weaver.Main]
	weaver.WithConfig[option] ` + "`conf:\"app\"`" + `
	weaver.Ref[app.HTTP]
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *Main) error {
		conf := a.Config()
		a.Logger(ctx).Info("app 启动成功", "name", conf.Name, "version", conf.Version)

		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		panic(err)
	}
}
`))

var appTmpl = template.Must(template.New("app").Parse(`package app

import (
	"context"
	"net/http"

	"github.com/jun3372/weaver"
)

type HTTP any

// httpx 是 HTTP 的组件实现:内嵌 Listener 后配置自动注入,
// Init 中注册 handler 即可,框架自动启动服务,无需编写 Start。
type httpx struct {
	weaver.Implements[HTTP]
	weaver.Listener[weaver.Handler] ` + "`conf:\"http\"`" + `
}

func (h *httpx) Init(ctx context.Context) error {
	// 接入 gin/echo/chi 等外部框架时,替换为对应引擎:
	// srv := gin.Default()
	// srv.GET("/hello", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"message": "hello"}) })
	// h.Listener.Handler(srv)

	srv := http.NewServeMux()
	srv.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Hello, Weaver!"))
	})
	h.Listener.Handler(srv)

	h.Logger(ctx).Debug("http 组件初始化完成", "conf", h.Listener.Config())
	return nil
}
`))

var configTmpl = template.Must(template.New("config").Parse(`app:
  Name: "{{.Name}}"
  Version: "v0.0.1"

http:
  Addr: ":8080"
  ReadTimeout: 5s
  WriteTimeout: 10s
  IdleTimeout: 120s

weaver:
  Logger:
    Level: "info"
    Type: "json"
`))

var makefileTmpl = template.Must(template.New("makefile").Parse(`APP     := {{.Name}}
MAIN    := ./cmd
CONFIG  := etc/weaver.yaml
IMAGE   := {{.Name}}:latest

.PHONY: run build generate test vet tidy docker-build compose-up compose-down k8s-apply k8s-delete

## run: 本地运行(热更新依赖配置文件原地写入)
run:
	go run $(MAIN) -conf $(CONFIG)

## build: 编译二进制到 bin/
build:
	go build -o bin/$(APP) $(MAIN)

## generate: 组件接口变更后重新生成注册代码
generate:
	weaver generate ./...

## test: 全量测试(含竞态检测)
test:
	go test -race ./...

vet:
	go vet ./...

tidy:
	go mod tidy

## docker-build: 构建应用镜像(构建上下文为项目根,-f 指定 cmd/Dockerfile)
docker-build:
	docker build -f cmd/Dockerfile -t $(IMAGE) .

## compose-up / compose-down: Docker Compose 部署
compose-up:
	docker compose -f deploy/docker-compose.yaml up -d --build

compose-down:
	docker compose -f deploy/docker-compose.yaml down

## k8s-apply / k8s-delete: Kubernetes 部署
k8s-apply:
	kubectl apply -f deploy/k8s.yaml

k8s-delete:
	kubectl delete -f deploy/k8s.yaml
`))

var dockerfileTmpl = template.Must(template.New("dockerfile").Parse(`# 构建阶段
FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd

# 运行阶段:优雅关闭依赖 SIGTERM,保留默认 stop 信号即可
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/app ./app
COPY etc ./etc
EXPOSE 8080
ENTRYPOINT ["./app", "-conf", "etc/weaver.yaml"]
`))

var composeTmpl = template.Must(template.New("compose").Parse(`services:
  app:
    build:
      context: ..
      dockerfile: cmd/Dockerfile
    image: {{.Name}}:latest
    container_name: {{.Name}}
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/"]
      interval: 30s
      timeout: 3s
      retries: 3
`))

var k8sTmpl = template.Must(template.New("k8s").Parse(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Name}}
  labels:
    app: {{.Name}}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{.Name}}
  template:
    metadata:
      labels:
        app: {{.Name}}
    spec:
      containers:
        - name: app
          image: {{.Name}}:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8080
          readinessProbe:
            httpGet:
              path: /
              port: 8080
            initialDelaySeconds: 2
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 20
          resources:
            requests:
              cpu: 100m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
---
apiVersion: v1
kind: Service
metadata:
  name: {{.Name}}
spec:
  type: ClusterIP
  selector:
    app: {{.Name}}
  ports:
    - port: 80
      targetPort: 8080
`))
