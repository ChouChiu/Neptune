# 涅普涅普的群组守护大作战！<img src="icon.jpg" align="right" width="30%" alt="Nepktune" style="margin-top: 20px;">

> 呀吼~指挥官！把群组交给涅普涅普就放心吧！(๑•̀ㅂ•́)و✧
> 
> 诶？你说为什么是海王星女神来管群组？因为人家是「主人公」啊！涅普♪

这是 Telegram 群组管理机器人 **Neptune** 的使用与部署说明。

**Neptune** - 穿越到 Telegram 群组的紫耀之都守护女神，用 Go 编写的轻量级群组管理 Bot！单二进制部署，纯 Go SQLite（WAL 模式）存储。

## 涅普的超能力菜单 (ﾉ>ω<)ﾉ

- **入群欢迎** - 自定义欢迎消息，支持 MarkdownV2 和占位符，自动清理入群提示并定时清理超时欢迎消息
- **入群认证** - 入群发送第 1 次验证提示（暂不禁言）；未验证成员首次在群内发言时自动删除其消息、立即禁言并发送第 2 次验证提示；验证提示在完成验证或超时后自动删除；支持内存实时生成 BMP 图片验证码、入群群规强制阅读倒计时、5 次错误自动锁定、双通道入群事件（`new_chat_members` + `chat_member`）去重与解除限制重试校验
- **自动回复** - 关键词与正则匹配自动回复（内置 `gocc` 繁简中文互匹配与 60s 规则缓存）
- **投票踢人** - 群成员通过 `/kick` 发起投票踢人（5 票通过、60s 发起冷却、过期自动清理）
- **管理员私聊管理** - 在群组发送 `/id` 或私聊 `/connect` 绑定群组，支持 `/switch` 多群组切换管理
- **举报与警告** - 群成员可通过 `/report` 举报违规消息，管理员可通过 `/warn` 记录警告
- **Web 管理面板** - 基于 Chi + Templ + HTMX + SSE 的管理后台（`/admin/`），使用 Telegram Login Widget 认证，支持举报审核（原子防重复处理）、警告记录查看与实时日志流
- **GitHub Release 通知** - 接收 `POST /github-webhook`（HMAC-SHA256 验签），将 GFM Release 说明安全转换为 Telegram MarkdownV2 推送到频道
- **权限控制** - 所有管理命令均校验群管理员身份或私聊绑定关系

## 涅普的小秘密 (various)

- 涅普最喜欢布丁了！写上「涅普的」防止被偷吃，但碧雪总会拿走写了名字的那个……
- 涅普讨厌茄子！一闻到茄子味就全身无力，所以群里请不要发茄子图片！
- 涅普是游戏高手！沉迷死宅，有什么好玩的游戏记得推荐给涅普哦！
- 涅普有主角光环！作为「主人公中的主人公」，运气总是好呢~
- 涅普绝不穿裙子！这是原则问题！（虽然女神化会变身……）
- 涅普的口癖是「涅普」！惊讶、被打、疑问、甚至睡觉呼噜都是「ねぷぅ……ねぷぅ……」

## 命令列表 ヽ(✿ﾟ▽ﾟ)ノ

### 通用

| 命令 | 说明 |
|------|------|
| `/help` | 显示帮助信息 |
| `/ping` | 检查机器人是否在线 |
| `/id` | 获取当前群组 ID（群管理员发送时自动关联群组） |

### 管理员（私聊使用）

| 命令 | 说明 |
|------|------|
| `/connect <群组ID>` | 绑定私聊与群组（需目标群组管理员权限） |
| `/switch` | 切换当前管理的群组 |

### 入群欢迎（需要管理员权限）

| 命令 | 说明 |
|------|------|
| `/setwelcome <消息>` | 设置欢迎消息（支持占位符） |
| `/enablewelcome` | 启用入群欢迎 |
| `/disablewelcome` | 禁用入群欢迎 |

### 入群认证（需要管理员权限）

| 命令 | 说明 |
|------|------|
| `/rule <内容>` | 设置群规（入群认证前需阅读 10 秒）；`/rule off` 清除群规 |
| `/setverifybutton <文案>` | 设置认证按钮文案 |
| `/setverifytimeout <秒>` | 设置认证超时时间（1–86400 秒） |
| `/testverify` | 在群组中测试欢迎验证消息 |

### 自动回复（需要管理员权限）

| 命令 | 说明 |
|------|------|
| `/addkeyword <关键词> <回复>` | 添加关键词规则 |
| `/addregex <正则> <回复>` | 添加正则规则（忽略大小写，最大 200 字符） |
| `/listkeywords` | 列出所有规则 |
| `/removekeyword <关键词>` | 删除指定规则 |

> 关键词和正则均支持繁简中文自动匹配，例如关键词为“学习”可匹配“學習”，反之亦然。

### 投票踢人

| 命令 | 说明 |
|------|------|
| `/enablevotekick` | 启用投票踢人（管理员） |
| `/disablevotekick` | 禁用投票踢人（管理员） |
| `/kick` | 回复目标用户消息发起踢人投票（群成员可用） |

### 举报与警告

| 命令 | 说明 |
|------|------|
| `/report <原因>` | 回复目标用户消息提交举报（群成员可用） |
| `/warn [原因]` | 回复目标用户消息进行警告（管理员可用） |

- **管理面板**：访问 `https://<你的域名>/admin/` 使用 Telegram 账号登录后，可审核举报（通过/驳回）、查看警告记录并实时查看服务端日志。
- **举报处理**：通过后自动删除被举报消息（若存在）、记录警告、在群组发送处理通知并私信通知举报人。

## 占位符魔法

欢迎消息和自动回复支持以下占位符：

| 占位符 | 说明 |
|--------|------|
| `{nickname}` | 用户昵称（MarkdownV2 模式下自动渲染为可点击用户链接） |
| `{userid}` | 用户 ID |
| `{groupname}` | 群组名称 |

## 使用流程

1. 将机器人加入群组并授予管理员权限（需包含删除消息与限制成员权限）。
2. 群管理员在群组中发送 `/id` 获取群组 ID（自动完成私聊管理绑定），或在私聊中使用 `/connect <群组ID>` 绑定。
3. 在私聊中发送 `/switch` 选择要管理的群组。
4. 使用命令配置欢迎消息、群规、认证超时和关键词自动回复，或访问 `/admin/` 管理举报与查看日志。

## 环境变量配置

完整模板见 [`.env.example`](.env.example)：

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `BOT_TOKEN` | 是 | - | Telegram Bot Token（来自 `@BotFather`） |
| `BOT_USERNAME` | 建议 | 空 | Bot 用户名（不含 `@`），用于注册 `cmd@username` 与验证链接 |
| `GITHUB_WEBHOOK_SECRET` | 建议 | 空 | GitHub Webhook HMAC-SHA256 密钥，同时作为 `/set-webhook` 鉴权 token |
| `RELEASE_CHANNEL_ID` | 否 | `0` | 接收 GitHub Release 通知的 Telegram 频道 ID |
| `LISTEN_ADDR` | 否 | `:8080` | HTTP 服务监听地址 |
| `DB_PATH` | 否 | `data/neptune.db` | SQLite 数据库文件路径 |

## 开发与部署

架构细节与贡献流程请参阅 [贡献指南](CONTRIBUTING.md)。

### 本地开发

```bash
# 下载依赖
go mod download

# 复制配置并填写 BOT_TOKEN 等参数
cp .env.example .env

# 生成 templ 模板并编译/测试
make generate
make build
make test             # 运行单元测试
make lint             # 运行 golangci-lint
make vet              # 运行 go vet
make dev              # 使用 air 热重载运行
```

### 部署到 VPS

完整步骤请参阅 [部署指南](DEPLOY.md)：

```bash
# 1. 服务器初始化（在 VPS 上执行）
ssh root@你的服务器 "bash -s" < deploy/setup.sh

# 2. 填写 /opt/neptune/.env 后构建启动
make docker-build
make docker-up

# 3. 注册 Telegram Webhook 并执行端到端检查
BOT_TOKEN=你的token make webhook DOMAIN=bot.example.com WEBHOOK_SECRET=你的密钥
BOT_TOKEN=你的token make e2e BASE_URL=https://bot.example.com
```

## License

MIT
