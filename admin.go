package main

import (
	"strings"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func isAdmin(uid int64) bool {
	// استبدل الرقم بالـ ID الحقيقي الخاص بك من @userinfobot
	return uid == 7938600557 
}

func handleAdminCommands(bot *tgbotapi.BotAPI, uid int64, text string) bool {
	if strings.HasPrefix(text, "حذف ") {
		sku := strings.ToUpper(strings.TrimPrefix(text, "حذف "))
		DB.Delete(&Product{}, "sku = ?", sku)
		sendMsg(bot, uid, "✅ تم الحذف.")
		return true
	}
	return false
}
