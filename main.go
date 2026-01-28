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

var DB *gorm.DB

func main() {
	// 1. إعداد سيرفر ويب وهمي لإرضاء Render Health Check
	// يعمل في Routine منفصل لكي لا يعطل البوت
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		
		// مسار بسيط للتحقق من الحالة
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Bot is running and healthy!")
		})

		log.Printf("Starting health check server on port %s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("Health check server failed: %v", err)
		}
	}()

	// 2. الاتصال بقاعدة البيانات Postgres
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// التأكد من وجود الجداول (AutoMigrate)
	err = DB.AutoMigrate(&Product{})
	if err != nil {
		log.Printf("Migration error: %v", err)
	}

	// 3. إعداد بوت تيليجرام
	botToken := os.Getenv("TELEGRAM_APITOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_APITOKEN environment variable is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	// ضبط وضع التصحيح (Debug) لمراقبة الرسائل في Logs
	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// 4. حلقة استقبال الرسائل ومعالجتها
	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text
		username := update.Message.From.UserName

		// الحصول على معرف الصورة إذا وجدت
		var photoID string
		if update.Message.Photo != nil {
			photoID = update.Message.Photo[len(update.Message.Photo)-1].FileID
		}

		// تمرير البيانات لمنطق البوت (الموجود في الملفات الأخرى مثل admin.go و logic.go)
		logicHandler(bot, chatID, text, photoID, username)
	}
}
