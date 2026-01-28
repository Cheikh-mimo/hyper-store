package main // هذا السطر هو الذي كان ينقصك!

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// --- نماذج البيانات ---
type Product struct {
	gorm.Model
	SKU         string `gorm:"uniqueIndex"`
	Category    string
	Name        string
	Price       string
	Description string
	PhotoIDs    string
	Seller      string
}

type User struct {
	gorm.Model
	TelegramID int64 `gorm:"uniqueIndex"`
	Username   string
}

var DB *gorm.DB

func main() {
	// 1. نظام تشغيل السيرفر (Health Check) لـ Render
	go func() {
		port := os.Getenv("PORT")
		if port == "" { port = "8080" }
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "Hyper Store Engine is Online") })
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}()

	// 2. الاتصال بقاعدة البيانات
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("فشل الاتصال بالقاعدة: %v", err)
	}
	DB.AutoMigrate(&Product{}, &User{})

	// 3. إعداد البوت
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil { log.Panic(err) }

	log.Printf("Bot %s is active!", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// تعبير منتظم للبحث عن الرموز المكونة من 4 أرقام
	skuRegex := regexp.MustCompile(`^\d{4}$`)

	for update := range updates {
		// معالجة الأزرار
		if update.CallbackQuery != nil {
			handleCallbacks(bot, update.CallbackQuery)
			continue
		}

		if update.Message == nil { continue }
		msg := update.Message
		text := msg.Text

		// الأوامر الرئيسية
		if text == "/start" || text == "مرحبا" {
			showMainMenu(bot, msg.Chat.ID)
		} else if skuRegex.MatchString(text) {
			sendProductView(bot, msg.Chat.ID, text)
		}
	}
}

// عرض القائمة الرئيسية
func showMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🛒 *Hyper Store* \nأهلاً بك في المتجر، اختر الفئة المرجوة:")
	msg.ParseMode = "Markdown"
	
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔥 Free Fire", "cat_Free Fire"),
			tgbotapi.NewInlineKeyboardButtonData("🔫 PUBG", "cat_PUBG"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚽ EFOOTBALL", "cat_EFOOTBALL"),
		),
	)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// معالجة ضغطات الأزرار
func handleCallbacks(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	bot.Request(tgbotapi.NewCallback(query.ID, ""))
	data := query.Data
	chatID := query.Message.Chat.ID

	if strings.HasPrefix(data, "cat_") {
		category := strings.TrimPrefix(data, "cat_")
		var products []Product
		DB.Where("category = ?", category).Find(&products)
		
		if len(products) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "🚫 لا توجد منتجات حالياً في هذا القسم."))
			return
		}
		for _, p := range products {
			sendProductView(bot, chatID, p.SKU)
		}
	} else if strings.HasPrefix(data, "buy_") {
		sku := strings.TrimPrefix(data, "buy_")
		notifyAdmin(bot, query.From, sku)
		bot.Send(tgbotapi.NewMessage(chatID, "✅ تم إرسال طلبك! سيقوم المشرف بالتواصل معك قريباً."))
	}
}

// عرض تفاصيل المنتج
func sendProductView(bot *tgbotapi.BotAPI, chatID int64, sku string) {
	var p Product
	if err := DB.Where("sku = ?", sku).First(&p).Error; err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ الرمز غير موجود."))
		return
	}

	photos := strings.Split(p.PhotoIDs, ",")
	msg := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(photos[0]))
	msg.Caption = fmt.Sprintf("📦 *%s*\n💵 السعر: %s\n🆔 الرمز: `%s`", p.Name, p.Price, p.SKU)
	msg.ParseMode = "Markdown"

	btn := tgbotapi.NewInlineKeyboardButtonData("🛒 شراء الآن", "buy_"+p.SKU)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
	bot.Send(msg)
}

// إشعار الأدمن
func notifyAdmin(bot *tgbotapi.BotAPI, user *tgbotapi.User, sku string) {
	adminIDStr := os.Getenv("MY_ADMIN_ID")
	adminID, _ := strconv.ParseInt(adminIDStr, 10, 64)

	msgText := fmt.Sprintf("🔔 *طلب شراء جديد!*\n📦 الرمز: %s\n👤 المشتري: @%s\n🆔 الآيدي: `%d`", 
		sku, user.UserName, user.ID)
	
	msg := tgbotapi.NewMessage(adminID, msgText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}
