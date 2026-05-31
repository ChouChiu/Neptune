import type { Bot } from "grammy";
import { replyOptions } from "../../shared/utils/reply";

const HELP_TEXT = `📋 命令列表

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

💡 占位符: {nickname} {userid} {groupname}`;

export function registerHelpCommand(bot: Bot): void {
	bot.command("help", async (ctx) => {
		await ctx.reply(HELP_TEXT, replyOptions(ctx));
	});
}
