# 打码平台集成 — 设计文档

> 日期: 2026-06-24 | 状态: 设计完成，待实现

## 1. 概述

为百度指数接入超级鹰 (Chaojiying) 打码服务，实现自动登录和运行时验证码处理，使百度指数从"需人工维护登录态"升级为"全自动采集"。

### 核心目标

- 定义 `CaptchaSolver` 接口，解耦打码服务商
- 实现超级鹰 HTTP 客户端
- 百度指数支持自动登录（Cookie 过期后自动重登）
- 运行时验证码检测 → 暂停采集 → 打码 → 恢复
- 识别失败自动重试（最多 2 次）

---

## 2. 架构

### 2.1 CaptchaSolver 接口

```go
// internal/engine/captcha.go

type CaptchaSolver interface {
    Solve(ctx context.Context, img []byte, captchaType string) (*CaptchaResult, error)
    ReportError(ctx context.Context, id string) error
}

type CaptchaResult struct {
    ID   string // 上报错误时用
    Code string // 识别出的验证码文本
}
```

### 2.2 超级鹰 API

```
POST http://upload.chaojiying.net/Upload/Processing.php

参数:
  user       账号
  pass       密码
  softid     软件ID (默认 96001)
  codetype   1902=中文汉字, 1004=数字字母
  userfile   验证码图片 (multipart file)
  len_min    0
  time_out   20

响应:
  {"err_no":0, "err_str":"OK", "pic_id":"xxx", "pic_str":"识别文本"}
```

### 2.3 登录流程

```
Cookie 有效? → 直接进入采集
     ↓ 无效/不存在
Rod 打开登录页
     ↓
截图验证码 → solver.Solve()
     ↓
填入账号+密码+验证码 → 提交
     ↓
检测登录成功? → Save Cookie → 进入采集
     ↓ 失败
ReportError → 重试 (最多2次)
```

### 2.4 运行时验证码

```
采集每 10 页检测一次验证码弹窗
     ↓ 检测到
截图 → solver.Solve() → 填入 → 提交
     ↓ 成功
继续采集
     ↓ 失败
重试 1 次 → 仍失败 → 标记 task error
```

---

## 3. 文件蓝图

| 文件 | 职责 | 变更 |
|------|------|------|
| `internal/engine/captcha.go` | CaptchaSolver 接口 | 新建 |
| `internal/engine/chaojiying.go` | 超级鹰客户端 | 新建 |
| `internal/engine/chaojiying_test.go` | 超级鹰测试 | 新建 |
| `internal/adapter/baidu_index/adapter.go` | 重写：登录+运行时验证码 | 修改 |
| `internal/adapter/baidu_index/adapter_test.go` | 扩展测试 | 修改 |
| `cmd/crawler/main.go` | 初始化 Solver，注入 adapter | 修改 |
| `config/config.yaml` | captcha 配置段 | 修改 |

---

## 4. 配置

```yaml
captcha:
  provider: chaojiying
  chaojiying:
    user: ${CAPTCHA_USER}
    password: ${CAPTCHA_PASS}
    soft_id: "96001"
```

---

## 5. 关键设计决策

| 决策 | 结论 | 理由 |
|------|------|------|
| 接口位置 | engine 包 | 与 RodPool/SessionManager 同级 |
| 重试次数 | 最多 2 次 | 超级鹰失败率 ~10%，2 次覆盖大部分场景 |
| 错误上报 | 失败后调用 ReportError | 超级鹰支持报错返现 |
| Cookie 检查 | 登录前先 Load 检查 | 减少不必要登录 |
| 验证码检测频率 | 每 10 页 | 平衡性能和及时性 |
| 打码类型 | codetype=1902 | 百度指数验证码主要为中文 |

---

## 6. 范围

### Phase 1（本次）
- 超级鹰集成
- 百度指数自动登录
- 百度指数运行时验证码处理

### 后续
- 图鉴 (Tujian) 接入
- 2captcha 接入
- 淘数据/生意参谋/抖音快手集成
- 登录态心跳检测
