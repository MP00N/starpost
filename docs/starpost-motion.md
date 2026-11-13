# 驿站动效规范

> 驿站是**通信隐喻**，不是古风皮肤。动效服务于「发生了什么」，不服务于氛围。

## 1. 一句话判据

一个动效要么回答「请求处于什么状态」，要么什么都不做。
回答不了这个问题的动效，删掉。

## 2. 频率分级（决定有没有动效）

| 频率 | 例子 | 动效预算 |
| --- | --- | --- |
| 高频 | 列表滚动、tab 切换、菜单展开、表格翻页、命令面板 | **零驿站动效**，只允许 ≤150ms 标准状态反馈（`--sp-dur-state`） |
| 低频 | 校验凭证、请求发出、结果落地、请求失败、环境切换、首次进入 | 允许具名动效（见 §3） |

## 3. 语义映射

| 事件 | 隐喻 | 实现 | 时长 / 曲线 |
| --- | --- | --- | --- |
| 凭证校验 | 验符入驿 | 棨信刻痕逐格点亮 → 通过则短弧入驿门 → 落小印 | 200–250ms `--sp-ease-enter` |
| 请求在途 | 驿路线 | 1px 线 `scaleX` 循环推进 | `linear` 无限；收束 150ms |
| 成功返回 | 驿印落定 | `scale(0.92)→1` + opacity，落定后停 | 140ms `--sp-ease-enter` |
| 失败/超时 | 驿路阻断 | `clip-path` 一次性切出锯齿断口，200ms 后线淡出 | 瞬时切换；淡出 200ms `--sp-ease-exit` |
| 环境切换 | 换马 | 旧标签横移淡出，新标签同向淡入 | 200ms `--sp-ease-travel` |
| 页面转场 | 过驿门 | 侧向 `translateX` + opacity，不用缩放/3D | 340ms（移动端 272ms） |
| 后台静默刷新 | 驿卒换班 | 极轻 opacity pulse，或无动效 | ≤200ms 或 none |

## 4. Token 表

定义在 `web/src/styles/starpost-motion.css`，全局唯一来源。

**时长**

| Token | 值 | 用途 |
| --- | --- | --- |
| `--sp-dur-touch` | 90ms | 触摸反馈 |
| `--sp-dur-micro` | 100ms | 开关、徽标 |
| `--sp-dur-state` | 150ms | 标准状态变化（高频） |
| `--sp-dur-enter` | 220ms | 小元素进入 |
| `--sp-dur-exit` | 200ms | 小元素离开 |
| `--sp-dur-swap` | 200ms | 换马 |
| `--sp-dur-gate` | 340ms | 过驿门 |

移动端（≤640px）整组 ×0.8，由同一段媒体查询覆盖，不改组件。

**曲线**

| Token | 值 | 用途 |
| --- | --- | --- |
| `--sp-ease-enter` | `cubic-bezier(0.23, 1, 0.32, 1)` | 出现 |
| `--sp-ease-exit` | `cubic-bezier(0.4, 0, 1, 1)` | 消失 |
| `--sp-ease-travel` | `cubic-bezier(0.77, 0, 0.175, 1)` | 屏内位移 |
| `--sp-ease-loop` | `linear` | 连续循环 |

## 5. 组件与挂载点

| 组件 | 文件 | 语义 | 挂载点 |
| --- | --- | --- | --- |
| `SealStamp` | `components/starpost/seal-stamp.tsx` | 驿印 | 请求结果区块的头部 |
| `RouteLine` | `components/starpost/route-line.tsx` | 驿路线 | 结果区顶部；订阅全局在途计数 |
| `RoadBreak` | `components/starpost/road-break.tsx` | 断口 | 请求失败处 |
| `OrderToken` | `components/starpost/order-token.tsx` | 棨信 | 凭证输入框左侧 |
| `MountSwap` | `components/starpost/mount-swap.tsx` | 换马 | 环境标签 |
| `PageTransition` / `AnimatedOutlet` | `components/page-transition.tsx` | 过驿门 | 已挂载（登录后路由容器） |

在途计数来自 `lib/request-activity.ts`，由 `lib/http-client.ts` 的一对拦截器打点。该拦截器**只计数**，不读改 config/response，因此不影响既有的刷新、重试与错误整形。

## 6. 硬约束

1. 所有动效只动 `transform` 和 `opacity`，**不碰** `width/height/top/left`。
2. 禁止：hover 抬升 / 阴影增长 / 弹跳回弹 / 全屏 3D 翻转 / 大面积水墨晕染 / 马旗灯笼乱飞。
3. 错误不得只靠动效传达：**错误码 + 原因 + 建议动作**必须同时存在且可读屏（`role="alert"`）。
4. `prefers-reduced-motion` 下：关闭位移、弧线、驿路线推进；仅保留 opacity 交叉淡化，时长减半。
5. 移动端禁用 parallax 与多层 stagger；驿路线跑不到 60fps 时降级为静态进度条 + 百分比数字。
6. 不引入重型动画库。Lottie 仅限「验符入驿」与「首次认证成功」两个节点，且必须 canvas 渲染、30fps、进出视口暂停。

## 7. 素材边界

| 允许 | 说明 |
| --- | --- |
| CSS `clip-path` | 印泥缺口、断口锯齿、云纹节奏，零素材 |
| CSS `mask-image` + 渐变 | 极轻云纹遮罩，不引图片 |
| IconPark（Apache 2.0） | 棨信、驿印外轮廓等基础形，仅取外轮廓 |
| 思源宋体 / 思源黑体（SIL OFL） | 正文与 UI |
| 黄令东齐伋体（SIL OFL） | 仅极少量标题肌理；印面 36px 下不可用 |

**不用**：来源不明的古风素材包、带背景的公章 PNG、需商业授权的书法字体、逐帧 GIF 云纹。
印面文字不使用篆书——36px 下不可读，控制台的印必须可读。

## 8. 自查清单

- [ ] 高频操作零驿站动效
- [ ] 只动 `transform` / `opacity`
- [ ] `prefers-reduced-motion` 生效（位移归零、时长减半）
- [ ] 移动端有降级（时长 ×0.8、无 parallax）
- [ ] 错误三要素齐全且可读屏
- [ ] 落定类动效结束后无持续动画
