package main

import (
	"embed"
	"net/http"
	"os"
	"time"

	"github.com/apex/httplog"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	"github.com/caarlos0/starcharts/config"
	"github.com/caarlos0/starcharts/controller"
	"github.com/caarlos0/starcharts/internal/cache"
	"github.com/caarlos0/starcharts/internal/github"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed static/*   这行代码表示将 static 目录下的所有文件嵌入到 static 变量中。然后，可以通过 static 来访问这些文件的内容。
var static embed.FS

var version = "devel"

func main() {
	log.SetHandler(text.New(os.Stderr)) //这是日志处理器，日志输出到标准错误输出
	// log.SetLevel(log.DebugLevel)
	config := config.Get()
	ctx := log.WithField("listen", config.Listen)
	options, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		log.WithError(err).Fatal("invalid redis_url")
	}
	redis := redis.NewClient(options) //创建一个redis客户端
	cache := cache.New(redis) //创建缓存，在main结束时结束缓存
	defer cache.Close()
	github := github.New(config, cache) //初始化 GitHub 客户端，传入配置和缓存实例。

	r := mux.NewRouter()  //创建一个路由器，用来定义一个路由
	r.Path("/").
		Methods(http.MethodGet).   //这里是get
		Handler(controller.Index(static, version))   //从static文件里面拿东西出来显示到网页上
	r.Path("/").
		Methods(http.MethodPost).   //这里是post
		HandlerFunc(controller.HandleForm())
	r.PathPrefix("/static/").
		Methods(http.MethodGet).
		Handler(http.FileServer(http.FS(static)))
	r.Path("/{owner}/{repo}.svg").
		Methods(http.MethodGet).
		Handler(controller.GetRepoChart(github, cache))
	r.Path("/{owner}/{repo}").
		Methods(http.MethodGet).
		Handler(controller.GetRepo(static, github, cache, version))

	// generic metrics   创建一个 Prometheus 计数器，用于统计请求总数，按状态码和请求方法分组
	requestCounter := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "starcharts",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "total requests",
	}, []string{"code", "method"})
	responseObserver := promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "starcharts",
		Subsystem: "http",
		Name:      "responses",
		Help:      "response times and counts",
	}, []string{"code", "method"})

	r.Methods(http.MethodGet).Path("/metrics").Handler(promhttp.Handler())

	srv := &http.Server{
		Handler: httplog.New(
			promhttp.InstrumentHandlerDuration(
				responseObserver,
				promhttp.InstrumentHandlerCounter(
					requestCounter,
					r,
				),
			),
		),
		Addr:         config.Listen,
		WriteTimeout: 60 * time.Second,
		ReadTimeout:  60 * time.Second,
	}
	ctx.Info("starting up...")
	ctx.WithError(srv.ListenAndServe()).Error("failed to start up server")
}
