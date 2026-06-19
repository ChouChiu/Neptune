package handler

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ChouChiu/neptune/internal/util"
)

const helpText = `📋 命令列表

🔹 通用
/help - 显示此帮助信息
/ping - 检查机器人是否在线
/id - 获取当前群组 ID

🔹 管理员
/connect <群组ID> - 绑定私聊与群组
/switch - 切换管理的群组（私聊）

🔹 入群欢迎
/setwelcome <消息> - 设置欢迎消息
/enablewelcome - 启用入群欢迎
/disablewelcome - 禁用入群欢迎

🔹 入群认证
/setverifybutton <文案> - 设置认证按钮文案
/setverifytimeout <秒> - 设置验证超时时间
/testverify - 测试验证消息
/rule <内容> - 设置群规（入群需阅读）

🔹 自动回复
/addkeyword <关键词> <回复> - 添加关键词规则
/addregex <正则> <回复> - 添加正则规则
/listkeywords - 列出所有规则
/removekeyword <关键词> - 删除规则

🔹 投票踢人
/enablevotekick - 启用投票踢人
/disablevotekick - 禁用投票踢人
/kick - 回复目标用户消息发起踢人投票

🔹 警告与举报
/warn [原因] - 警告用户（回复目标消息）
/report <内容> - 举报用户（回复目标消息）

🌐 后台管理: 访问 /admin 查看和处理举报及警告

💡 占位符: {nickname} {userid} {groupname}`

// Help returns a handler that responds with the help text.
func Help() tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            helpText,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}
