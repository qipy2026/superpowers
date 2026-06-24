# 网络数据爬虫系统 — 设计文档

> 日期: 2026-06-24 | 状态: 设计完成，待实现

## 1. 概述

一个 Go 语言编写的网络数据爬虫系统，覆盖 18 个中国数据平台，MySQL 存储 + RESTful API 查询，支持定时调度和手动触发。

### 目标平台（分阶段）

| 阶段 | 平台 | 复杂度 | 策略 |
|------|------|--------|------|
| Phase 1 | 国家统计局、行业排行榜 | 低 | Colly + 静态页面 |
| Phase 2 | 艾瑞数据、骨朵数据、猫眼票房、企鹅智库、腾讯研究院 | 中 | Colly + 可能需 JS 渲染 |
| Phase 3 | 清博、新榜、百度指数、淘数据、生意参谋、抖音快手网、火烧云、九九抖商 | 高 | Rod/Playwright + 认证 |

### 技术栈

| 组件 | 选型 |
|------|------|
| 语言 | Go 1.22+ |
| HTTP 爬虫 | `gocolly/colly` |
| 动态页面 | `go-rod/rod`（Phase 2+） |
| Web 框架 | `gin-gonic/gin` |
| ORM | `gorm.io/gorm` + MySQL driver |
| 定时任务 | `robfig/cron/v3` |
| 配置 | `spf13/viper` |
| 日志 | `uber-go/zap` |

---

## 2. 架构

### 2.1 整体架构 — 单体引擎 + Adapter 模式

```
main.go
├── API Server (Gin)
│   ├── GET  /api/v1/adapters
│   ├── POST /api/v1/crawl
│   ├── GET  /api/v1/crawl/:task_id
│   ├── GET  /api/v1/data
│   └── GET  /api/v1/stats
├── Scheduler (robfig/cron)
│   └── Cron 表达式驱动，周期性调用 Engine
├── Engine
│   ├── RateLimiter      — token bucket，per-adapter + global
│   ├── RetryManager     — 指数退避，1s→2s→4s→8s→16s，最多3次
│   ├── ProxyPool        — Phase 1 空实现，Phase 2+ 接入
│   └── ResultPipeline   — 采集结果批量写入 MySQL
├── Adapter Registry
│   └── 18 个 Adapter，每个实现 Adapter 接口
└── Persistence Layer (GORM + MySQL)
```

### 2.2 目录结构

```
superpowers/
├── cmd/crawler/main.go
├── internal/
│   ├── api/
│   │   ├── handler.go
│   │   └── router.go
│   ├── engine/
│   │   ├── engine.go
│   │   ├── limiter.go
│   │   ├── retry.go
│   │   └── proxy.go
│   ├── adapter/
│   │   ├── adapter.go          # Adapter 接口定义
│   │   ├── registry.go
│   │   ├── stats_gov/          # 国家统计局
│   │   ├── hangye_paihang/     # 行业排行榜
│   │   └── ...
│   ├── scheduler/scheduler.go
│   ├── model/model.go
│   └── repository/repository.go
├── config/config.yaml
├── docs/superpowers/specs/
├── go.mod
└── go.sum
```

---

## 3. Adapter 接口 & 数据模型

### 3.1 Adapter 接口

```go
type Task struct {
    ID        string    `json:"id"`
    Adapter   string    `json:"adapter"`
    Status    string    `json:"status"`    // pending | running | done | failed
    StartedAt time.Time `json:"started_at"`
    EndedAt   time.Time `json:"ended_at,omitempty"`
    Error     string    `json:"error,omitempty"`
    Stats     Stats     `json:"stats"`
}

type Stats struct {
    Collected int `json:"collected"`
    Skipped   int `json:"skipped"`
    Errors    int `json:"errors"`
}

type Adapter interface {
    Name() string
    Validate() error
    Collect(ctx context.Context, task *Task) ([]DataRow, error)
}
```

Collect 标准流程：检查 ctx 超时 → 请求入口页面 → 发现数据链接/分页 → 逐页采集 → 解析为 DataRow → 返回。

### 3.2 数据库表

**tasks** — 采集任务记录
| 列 | 类型 | 说明 |
|----|------|------|
| id | INT PK AUTO_INCREMENT | |
| adapter | VARCHAR(50) | 适配器名称 |
| status | VARCHAR(20) | pending/running/done/failed |
| error | TEXT | 错误信息 |
| stats_json | JSON | {collected, skipped, errors} |
| started_at | DATETIME | |
| ended_at | DATETIME | |

**data_rows** — 采集数据行
| 列 | 类型 | 说明 |
|----|------|------|
| id | INT PK AUTO_INCREMENT | |
| task_id | INT FK → tasks.id | |
| adapter | VARCHAR(50) | |
| source_url | VARCHAR(500) | |
| data_json | JSON | 实际数据，字段因平台而异 |
| collected_at | DATETIME | |
| checksum | CHAR(32) | MD5，去重和增量更新 |

**crawl_configs** — 调度配置
| 列 | 类型 | 说明 |
|----|------|------|
| adapter | VARCHAR(50) PK | |
| cron_expr | VARCHAR(50) | Cron 表达式，空=仅手动 |
| enabled | BOOLEAN | |
| rate_limit | INT | 每秒最大请求数 |
| updated_at | DATETIME | |

**设计理由：**
- `data_json` 使用 MySQL JSON 列：爬虫数据字段因平台而异，JSON 天然适配异构数据
- `checksum`：同一 URL + 同一 data 的 MD5 不重复写入

---

## 4. 错误处理 & 重试

### 4.1 重试策略

指数退避：1s → 2s → 4s → 8s → 16s，最多 3 次。

- **可重试**: timeout, 5xx, connection refused
- **不重试**: 404（页面不存在/下线）, 403（被明确封禁，需人工介入）, 解析错误（HTML 结构变更）

### 4.2 速率控制

Token bucket 实现，per-adapter + global 双层：
- 单个 adapter 独立限流（如 stats_gov: 5 req/s）
- 全局限流 50 req/s 保护出口带宽

### 4.3 代理池

Phase 1 为空实现。Phase 2+ 接入代理轮换，支持标记失效代理。

### 4.4 采集策略

| 策略 | 适用 | 实现 |
|------|------|------|
| 全量采集 | 历史数据一次性拉完 | 分页遍历到底，checksum 去重 |
| 增量采集 | 只拉最新数据 | 对比最新 collected_at 判断新增 |
| 监控采集 | 数据异常告警（后续扩展） | 增量 + 阈值比较 |

Phase 1 实现全量和增量。

---

## 5. API 设计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/adapters | 列出所有适配器及状态 |
| POST | /api/v1/crawl | 手动触发采集（202 Accepted） |
| GET | /api/v1/crawl/:task_id | 查询任务状态和进度 |
| GET | /api/v1/data | 查询采集数据（支持筛选/分页/搜索） |
| GET | /api/v1/stats | 统计概览 |

查询参数示例：
```
GET /api/v1/data?adapter=stats_gov&from=2024-01-01&to=2024-12-31&keyword=GDP&page=1&page_size=100
```

### 请求流程

```
POST /crawl → 校验 adapter → 创建 Task → 提交异步任务 (goroutine)
                                              │
                                   ┌──────────┴──────────┐
                                   │  Engine.Collect()   │
                                   └─────────────────────┘
                                              │
GET /crawl/:task_id ←────────────────────────┘ 实时查进度
```

---

## 6. 配置

```yaml
server:
  port: 8080
  mode: debug

database:
  host: 127.0.0.1
  port: 3306
  user: crawler
  password: ${CRAWLER_DB_PASSWORD}
  name: crawler_db
  max_open_conns: 25

engine:
  global_rate_limit: 50
  retry:
    max_retries: 3
    base_delay: 1s
    max_delay: 30s

adapters:
  - name: stats_gov
    enabled: true
    cron: "0 8 * * 1"
    rate_limit: 5
    mode: full
  - name: hangye_paihang
    enabled: true
    cron: "0 9 * * *"
    rate_limit: 10
    mode: incremental
```

---

## 7. Phase 1 范围

### 包含
- 项目骨架搭建（目录结构、配置加载、日志初始化）
- Engine 核心（RateLimiter, RetryManager）
- Scheduler（Cron + 手动触发）
- 2 个 Adapter：国家统计局（全量）、行业排行榜（增量）
- API 5 个端点完整实现
- MySQL 三张表 + Repository 层

### 不包含
- 动态页面平台（需 Rod）
- 需登录平台（需 session 管理）
- 代理池
- 可视化看板
- 数据异常告警

### 预估
- 约 15-20 个 Go 源文件
- 2 个 adapter 实现

---

## 8. Phase 2/3 展望

- Phase 2：接入 Rod 支持 JS 渲染页面，适配 5 个中等平台
- Phase 3：Session 管理 + Cookie 持久化，适配 8 个高难度平台，代理池正式接入
- 后续可能：数据可视化看板、导出功能、告警规则
