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

// --- القائمة الرئيسية ---
func getMainMenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 بيع منتج", "menu_sell"),
			tgbotapi.NewInlineKeyboardButtonData("🛒 شراء منتج", "menu_buy"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 بحث بالرمز", "menu_search"),
			tgbotapi.NewInlineKeyboardButtonData("📦 آخر المنتجات", "menu_latest"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎫 استخدام الكود", "menu_code"),
			tgbotapi.NewInlineKeyboardButtonData("🎁 استبدال النقاط", "menu_redeem"),
		),
	)
}

// --- قائمة الفئات (تظهر عند الضغط على بيع) ---
func getCategoryMenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎮 Free Fire", "cat_ff"),
			tgbotapi.NewInlineKeyboardButtonData("🔫 PUBG", "cat_pubg"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚽ EFOOTBALL", "cat_ef"),
			tgbotapi.NewInlineKeyboardButtonData("💳 Google Play", "cat_gp"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 رجوع", "go_back"),
		),
	)
}

func main() {
	go func() {
		port := os.Getenv("PORT")
		if port == "" { port = "8080" }
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "Bot Active") })
		http.ListenAndServe(":"+port, nil)
	}()

	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { log.Fatal(err) }

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil { log.Panic(err) }

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			handleCallbacks(bot, update.CallbackQuery)
			continue
		}

		if update.Message != nil && update.Message.Text == "/start" {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "مرحباً بك! يرجى اختيار زر من هذه الأزرار:")
			msg.ReplyMarkup = getMainMenu()
			bot.Send(msg)
		}
	}
}

func handleCallbacks(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	callbackCfg := tgbotapi.NewCallback(query.ID, "")
	bot.Request(callbackCfg)

	var editMsg tgbotapi.EditMessageTextConfig

	switch query.Data {
	case "menu_sell":
		// هنا نقوم بتعديل الرسالة بدلاً من إرسال واحدة جديدة
		editMsg = tgbotapi.NewEditMessageText(chatID, messageID, "يرجى تحديد الفئة الخاصة بالمنتج:")
		menu := getCategoryMenu()
		editMsg.ReplyMarkup = &menu

	case "go_back":
		// العودة للقائمة الرئيسية
		editMsg = tgbotapi.NewEditMessageText(chatID, messageID, "مرحباً بك مجدداً! يرجى الاختيار:")
		menu := getMainMenu()
		editMsg.ReplyMarkup = &menu

	case "cat_ff":
		editMsg = tgbotapi.NewEditMessageText(chatID, messageID, "ممتاز! لقد اخترت Free Fire. أرسل الآن السعر بالعملتين DA أو $:")
		// يمكن إضافة زر "إلغاء" هنا أيضاً
	
	default:
		editMsg = tgbotapi.NewEditMessageText(chatID, messageID, "عذراً، هذا القسم قيد التطوير.")
	}

	bot.Send(editMsg)
}
