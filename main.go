package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 1. تعريف موديل قاعدة البيانات
type Product struct {
	gorm.Model
	SKU         string `gorm:"unique"`
	Name        string
	Description string
	Price       string
	PhotoID     string
	Seller      string
	Reserved    bool
}

var DB *gorm.DB

// 2. دالة إرسال الرسائل المختصرة
func sendMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}

func main() {
	// سيرفر وهمي لإرضاء Render
	go func() {
		port := os.Getenv("PORT")
		if port == "" { port = "8080" }
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Bot is alive!")
		})
		http.ListenAndServe(":"+port, nil)
	}()

	// الاتصال بالقاعدة
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	DB.AutoMigrate(&Product{})

	// تشغيل البوت
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		log.Panic(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { continue }
		
		chatID := update.Message.Chat.ID
		text := update.Message.Text
		
		// منطق بسيط للتجربة (logicHandler)
		if text == "/start" || text == "مرحبا" {
			sendMsg(bot, chatID, "أهلاً بك في متجر الهيبر! البوت يعمل الآن بنجاح ✅")
		} else {
			sendMsg(bot, chatID, "وصلت رسالتك: " + text)
		}
	}
}
