import "strconv" // تأكد من إضافة هذه المكتبة في أعلى الملف مع الـ imports

func notifyAdmin(bot *tgbotapi.BotAPI, user *tgbotapi.User, sku string) {
	adminIDStr := os.Getenv("MY_ADMIN_ID")
	if adminIDStr == "" {
		log.Println("⚠️ تنبيه: لم يتم ضبط MY_ADMIN_ID في إعدادات Render")
		return
	}

	// تحويل النص إلى رقم
	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Println("❌ خطأ: آيدي الأدمن غير صحيح، يجب أن يكون أرقاماً فقط")
		return
	}

	msgText := fmt.Sprintf("🔔 *طلب شراء جديد!*\n📦 رمز المنتج: `%s`\n👤 المشتري: @%s\n🆔 آيدي المشتري: `%d`", 
		sku, user.UserName, user.ID)
	
	msg := tgbotapi.NewMessage(adminID, msgText)
	msg.ParseMode = "Markdown"
	
	bot.Send(msg)
}
