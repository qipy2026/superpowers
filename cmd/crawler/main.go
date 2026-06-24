package main

import (
    "fmt"
    "strings"
    "time"

    "crawler/internal/adapter"
    "crawler/internal/adapter/baidu_index"
    "crawler/internal/adapter/data_navi"
    "crawler/internal/adapter/douyin_kuaishou"
    "crawler/internal/adapter/guduo"
    "crawler/internal/adapter/hangye_paihang"
    "crawler/internal/adapter/haosou"
    "crawler/internal/adapter/huoshaoyun"
    "crawler/internal/adapter/iresearch"
    "crawler/internal/adapter/jiujiu_doushang"
    "crawler/internal/adapter/maoyan"
    "crawler/internal/adapter/penguin_intelligence"
    "crawler/internal/adapter/qingbo"
    "crawler/internal/adapter/shengyi_canmou"
    "crawler/internal/adapter/shudu"
    "crawler/internal/adapter/stats_gov"
    "crawler/internal/adapter/tao_data"
    "crawler/internal/adapter/tencent_research"
    "crawler/internal/adapter/xinbang"
    "crawler/internal/api"
    "crawler/internal/engine"
    "crawler/internal/repository"
    "crawler/internal/scheduler"

    "github.com/gin-gonic/gin"
    "github.com/spf13/viper"
    "go.uber.org/zap"
)

type AdapterCfg struct {
    Name      string `mapstructure:"name"`
    Enabled   bool   `mapstructure:"enabled"`
    Cron      string `mapstructure:"cron"`
    RateLimit int    `mapstructure:"rate_limit"`
    Mode      string `mapstructure:"mode"`
    BaseURL   string `mapstructure:"base_url"`
}

type Config struct {
    Server struct {
        Port int    `mapstructure:"port"`
        Mode string `mapstructure:"mode"`
    } `mapstructure:"server"`
    Database struct {
        Host         string `mapstructure:"host"`
        Port         int    `mapstructure:"port"`
        User         string `mapstructure:"user"`
        Password     string `mapstructure:"password"`
        Name         string `mapstructure:"name"`
        MaxOpenConns int    `mapstructure:"max_open_conns"`
    } `mapstructure:"database"`
    Engine struct {
        GlobalRateLimit float64 `mapstructure:"global_rate_limit"`
        Retry           struct {
            MaxRetries int    `mapstructure:"max_retries"`
            BaseDelay  string `mapstructure:"base_delay"`
            MaxDelay   string `mapstructure:"max_delay"`
        } `mapstructure:"retry"`
        ProxyURLs []string `mapstructure:"proxy_urls"`
    } `mapstructure:"engine"`
    Adapters []AdapterCfg `mapstructure:"adapters"`
}

func main() {
    logger, _ := zap.NewDevelopment()
    defer logger.Sync()

    // --- Config ---
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("config")
    viper.AddConfigPath(".")
    viper.SetEnvPrefix("CRAWLER")
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    viper.AutomaticEnv()
    if err := viper.ReadInConfig(); err != nil {
        logger.Fatal("failed to read config", zap.Error(err))
    }
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        logger.Fatal("failed to unmarshal config", zap.Error(err))
    }

    // --- Database ---
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
        cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
    repo, err := repository.New(dsn)
    if err != nil {
        logger.Fatal("failed to connect database", zap.Error(err))
    }
    if err := repo.AutoMigrate(); err != nil {
        logger.Fatal("failed to auto migrate", zap.Error(err))
    }

    // --- Infrastructure ---
    // Rod pool
    rodPool := engine.NewRodPool(2)
    if err := rodPool.Start(); err != nil {
        logger.Warn("Rod browser pool not available, JS adapters will fail", zap.Error(err))
    } else {
        defer rodPool.Close()
        logger.Info("Rod browser pool started")
    }
    // Session manager
    sessionMgr := engine.NewSessionManager("config/sessions")
    // Anti-detection
    antiDetect := engine.NewAntiDetect(nil)
    // Proxy pool
    proxyPool := engine.NewProxyPool(cfg.Engine.ProxyURLs)

    // --- Adapter Registry ---
    reg := adapter.NewRegistry()
    reg.Register(stats_gov.New("https://www.stats.gov.cn/sj/"))
    reg.Register(hangye_paihang.New("https://www.example.com/industry-ranking"))
    // Phase 2
    reg.Register(iresearch.New(getBaseURL(cfg.Adapters, "iresearch"), rodPool))
    reg.Register(guduo.New(getBaseURL(cfg.Adapters, "guduo"), rodPool))
    reg.Register(maoyan.New(getBaseURL(cfg.Adapters, "maoyan"), rodPool))
    reg.Register(penguin_intelligence.New(getBaseURL(cfg.Adapters, "penguin_intelligence")))
    reg.Register(tencent_research.New(getBaseURL(cfg.Adapters, "tencent_research")))
    // Phase 3
    reg.Register(qingbo.New(getBaseURL(cfg.Adapters, "qingbo"), rodPool, sessionMgr, antiDetect, proxyPool))
    reg.Register(xinbang.New(getBaseURL(cfg.Adapters, "xinbang"), rodPool, sessionMgr, antiDetect, proxyPool))
    reg.Register(baidu_index.New(getBaseURL(cfg.Adapters, "baidu_index"), rodPool, sessionMgr, antiDetect, proxyPool))
    reg.Register(tao_data.New(getBaseURL(cfg.Adapters, "tao_data"), rodPool, sessionMgr, antiDetect, proxyPool))
    reg.Register(shengyi_canmou.New(getBaseURL(cfg.Adapters, "shengyi_canmou"), rodPool, sessionMgr, antiDetect, proxyPool))
    reg.Register(douyin_kuaishou.New(getBaseURL(cfg.Adapters, "douyin_kuaishou"), rodPool, sessionMgr, antiDetect, proxyPool))
    reg.Register(huoshaoyun.New(getBaseURL(cfg.Adapters, "huoshaoyun"), rodPool, sessionMgr, antiDetect, proxyPool))
    reg.Register(jiujiu_doushang.New(getBaseURL(cfg.Adapters, "jiujiu_doushang"), rodPool, sessionMgr, antiDetect, proxyPool))
    // Final 3 low-complexity
    reg.Register(data_navi.New(getBaseURL(cfg.Adapters, "data_navi")))
    reg.Register(haosou.New(getBaseURL(cfg.Adapters, "haosou")))
    reg.Register(shudu.New(getBaseURL(cfg.Adapters, "shudu")))

    // --- Engine ---
    limiter := engine.NewMultiLimiter(cfg.Engine.GlobalRateLimit)
    for _, aCfg := range cfg.Adapters {
        if aCfg.RateLimit > 0 {
            limiter.SetAdapterLimit(aCfg.Name, float64(aCfg.RateLimit))
        }
    }
    retryMgr := engine.NewRetryManager(engine.RetryConfig{
        MaxRetries: cfg.Engine.Retry.MaxRetries,
        BaseDelay:  parseDuration(cfg.Engine.Retry.BaseDelay, "1s"),
        MaxDelay:   parseDuration(cfg.Engine.Retry.MaxDelay, "30s"),
    })
    eng := engine.New(repo, reg, limiter, retryMgr)
    eng.SetLogger(logger)

    // --- Scheduler ---
    sched := scheduler.New(eng, logger)
    for _, aCfg := range cfg.Adapters {
        if aCfg.Enabled && aCfg.Cron != "" {
            if err := sched.AddJob(aCfg.Name, aCfg.Cron); err != nil {
                logger.Error("failed to add cron job", zap.String("adapter", aCfg.Name), zap.Error(err))
            }
        }
    }
    sched.Start()

    // --- API ---
    if cfg.Server.Mode == "release" {
        gin.SetMode(gin.ReleaseMode)
    }
    router := gin.Default()
    handler := api.NewHandler(eng, repo, sched, reg, logger)
    api.RegisterRoutes(router, handler)

    addr := fmt.Sprintf(":%d", cfg.Server.Port)
    logger.Info("server starting", zap.String("addr", addr))
    if err := router.Run(addr); err != nil {
        logger.Fatal("server failed", zap.Error(err))
    }
}

func parseDuration(s, defaultVal string) time.Duration {
    d, err := time.ParseDuration(s)
    if err != nil {
        d, _ = time.ParseDuration(defaultVal)
    }
    return d
}

func getBaseURL(adapters []AdapterCfg, name string) string {
    for _, a := range adapters {
        if a.Name == name && a.BaseURL != "" {
            return a.BaseURL
        }
    }
    return ""
}
