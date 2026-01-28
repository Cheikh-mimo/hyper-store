package main

import (
	"fmt"
	"strings"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ضع الـ ID الخاص بك هنا (تحصل عليه بإرسال رسالة للبوت ورؤية الـ Logs)
var MyID int64 = 123456789 

func isAdmin(uid int64) bool {
	return uid == MyID
}

func handleAdminCommands(bot *tgbotapi.BotAPI, uid int64, text string) bool {
	if strings.HasPrefix(text, "حذف ") {
		sku := strings.ToUpper(strings.TrimPrefix(text, "حذف "))
		DB.Delete(&Product{}, "sku = ?", sku)
		sendMsg(bot, uid, "✅ تم حذف المنتج "+sku)
		return true
	}
	if text == "لوحة التحكم" {
		sendMsg(bot, uid, "أهلاً يا شيخ! 👑\nأوامرك:\n- حذف SKU\n- حجز SKU")
		return true
	}
	return false
}
