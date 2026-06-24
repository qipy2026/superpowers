# 数据可视化看板 — 设计文档

> 日期: 2026-06-24 | 状态: 设计完成，待实现

## 1. 概述

为 18 个爬虫平台构建 Web 数据可视化看板，使用 Go 模板 + HTMX 服务端渲染，Chart.js 图表，零前端依赖，编译为单一二进制部署。

### 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| 前端框架 | HTMX | 无 JS 框架依赖，属性驱动交互 |
| 模板引擎 | Go html/template | 标准库，零依赖 |
| 图表 | Chart.js (CDN) | 轻量，canvas 渲染 |
| 样式 | 纯 CSS | YAGNI，不引入 Tailwind |
| 部署 | embed.FS 嵌入 | 编译为单文件 |

---

## 2. 页面设计

### 2.1 概览首页

- 顶部 4 个统计卡片：平台总数 / 今日已采集 / 数据总量 / 采集异常
- 平台状态网格（18 张卡片），按类别分组，颜色标识状态
- HTMX 每 30s 自动刷新统计区域

### 2.2 平台详情页

- 顶部状态栏：总条数 / 上次采集状态 / 定时计划
- 图表区：Chart.js 柱状图（7 天趋势）+ 饼图（类别分布）
- 数据表：HTMX 分页，每页 20 条
- 操作按钮：手动采集（HTMX POST）+ CSV 导出

---

## 3. 文件蓝图

| 文件 | 职责 |
|------|------|
| `web/templates/base.html` | 公共布局（导航/header/CSS/Chart.js CDN） |
| `web/templates/overview.html` | 概览首页模板 |
| `web/templates/detail.html` | 平台详情页模板 |
| `web/static/style.css` | 样式表 |
| `internal/api/web_handler.go` | 页面渲染 + CSV 导出 handler |
| `internal/api/router.go` | 注册页面路由（修改） |
| `cmd/crawler/main.go` | embed 静态资源（修改） |

---

## 4. 路由

```
GET  /                        → overview.html
GET  /detail/:adapter         → detail.html
GET  /export/:adapter         → CSV 下载
# 已有 API 不变
GET  /api/v1/adapters
GET  /api/v1/data
GET  /api/v1/stats
POST /api/v1/crawl
GET  /api/v1/crawl/:id
```

---

## 5. 范围

### Phase 1（本次）
- 概览首页（统计卡片 + 平台网格）
- 平台详情页（图表 + 数据表 + 采集/导出）
- CSV 导出

### 不包含
- 用户认证/登录
- 实时 WebSocket 推送
- 告警通知
- 移动端适配

---

## 6. 预估

- 7 个文件（5 新建 + 2 修改）
- 约 500 行代码
