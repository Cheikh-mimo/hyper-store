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

// --- الهياكل البيانات ---
type User struct {
	gorm.Model
	TelegramID int64 `gorm:"uniqueIndex"`
	Username   string
	Points     int `gorm:"default:0"`
}

type Product struct {
	gorm.Model
	SKU      string `gorm:"uniqueIndex"`
	Name     string
	Category string
	Price    string
	Seller   string
}

var DB *gorm.DB

// --- لوحة الأزرار الرئيسية ---
func getMainMenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 بيع منتج", "sell"),
			tgbotapi.NewInlineKeyboardButtonData("🛒 شراء منتج", "buy"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 بحث بالرمز", "search"),
			tgbotapi.NewInlineKeyboardButtonData("📦 آخر المنتجات", "latest"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎫 استخدام الكود", "use_code"),
			tgbotapi.NewInlineKeyboardButtonData("🎁 استبدال النقاط", "redeem"),
		),
	)
}

func main() {
	// 1. تشغيل سيرفر الويب لـ Render
	go func() {
		port := os.Getenv("PORT")
		if port == "" { port = "8080" }
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "Bot Active") })
		http.ListenAndServe(":"+port, nil)
	}()

	// 2. الاتصال بقاعدة البيانات
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("خطأ اتصال: %v", err)
	}

	// 🔥 الحل السحري: حذف الجداول القديمة تماماً لإنهاء تعارض الـ SKU
	// ملاحظة: احذف هذين السطرين بعد أول تشغيل ناجح لكي لا تفقد بياناتك لاحقاً
	DB.Migrator().DropTable(&Product{}, &User{}) 
	log.Println("تم تنظيف الجداول القديمة بنجاح")

	// 3. إنشاء الجداول من جديد بالخصائص الصحيحة
	DB.AutoMigrate(&User{}, &Product{})

	// 4. تشغيل البوت
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil { log.Panic(err) }

	log.Printf("تم التشغيل على حساب: %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// معالجة الضغط على الأزرار
		if update.CallbackQuery != nil {
			handleCallbacks(bot, update.CallbackQuery)
			continue
		}

		if update.Message == nil { continue }

		// معالجة الرسائل
		if update.Message.Text == "/start" {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "مرحباً بك! يرجى اختيار زر من هذه الأزرار:")
			msg.ReplyMarkup = getMainMenu()
			bot.Send(msg)
		}
	}
}

func handleCallbacks(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	callbackCfg := tgbotapi.NewCallback(query.ID, "")
	bot.Request(callbackCfg)

	var text string
	switch query.Data {
	case "sell": text = "لقد اخترت: بيع منتج. يرجى إرسال تفاصيل المنتج."
	case "buy": text = "جاري عرض قائمة المنتجات..."
	case "search": text = "أدخل رمز البحث الخاص بك:"
	case "latest": text = "إليك آخر المنتجات المضافة."
	case "use_code": text = "أدخل كود الشحن:"
	case "redeem": text = "نقاطك الحالية 0. اجمع 1000 نقطة للاستبدال."
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, text)
	bot.Send(msg)
}
