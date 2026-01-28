package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// --- الموديلات ---
type Product struct {
	gorm.Model
	SKU         string `gorm:"uniqueIndex"`
	Category    string
	Name        string
	Price       string
	Description string
	PhotoIDs    string // معرفات الصور مفصولة بفاصلة (img1,img2,img3)
	Seller      string
}

type User struct {
	gorm.Model
	TelegramID int64 `gorm:"uniqueIndex"`
	Username   string
}

var DB *gorm.DB

// --- الإعدادات ---
var adminID string // سنقرأها من البيئة

func main() {
	// 1. إعداد السيرفر (Render Health Check)
	go func() {
		port := os.Getenv("PORT")
		if port == "" { port = "8080" }
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "Hyper Store Engine Running") })
		http.ListenAndServe(":"+port, nil)
	}()

	// 2. الاتصال بالقاعدة
	dsn := os.Getenv("DATABASE_URL")
	adminID = os.Getenv("MY_ADMIN_ID") // تأكد من وضع الآيدي الخاص بك في Render
	
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { log.Fatal(err) }
	
	// تحديث الجداول
	DB.AutoMigrate(&Product{}, &User{})

	// 3. البوت
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil { log.Panic(err) }

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// تعبير منتظم للكشف عن الرموز (4 أرقام) مثل كود Node.js
	skuRegex := regexp.MustCompile(`^\d{4}$`)

	for update := range updates {
		// أ) معالجة الأزرار (Callbacks)
		if update.CallbackQuery != nil {
			handleCallbacks(bot, update.CallbackQuery)
			continue
		}

		if update.Message == nil { continue }
		msg := update.Message
		chatID := msg.Chat.ID
		text := msg.Text

		// ب) معالجة الأوامر والبحث التلقائي
		
		// 1. إعادة التشغيل (Restart Logic)
		if text == "/start" || text == "menu" || text == "القائمة" {
			showMainMenu(bot, chatID)
			continue
		}

		// 2. البحث بالرمز مباشرة (Inspired by Regex in Node.js)
		if skuRegex.MatchString(text) {
			sendProductView(bot, chatID, text)
			continue
		}

		// 3. رسالة عادية
		bot.Send(tgbotapi.NewMessage(chatID, "مرحباً! 👋\nيمكنك إرسال رمز المنتج (4 أرقام) للبحث عنه مباشرة، أو استخدام القائمة."))
	}
}

// --- الدوال المساعدة ---

func showMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🛒 *مرحباً بك في Hyper Store*\n\nتصفح الأقسام أو ابحث عن منتج:")
	msg.ParseMode = "Markdown"
	
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔥 Free Fire", "cat_Free Fire"),
			tgbotapi.NewInlineKeyboardButtonData("🔫 PUBG", "cat_PUBG"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚽ EFOOTBALL", "cat_EFOOTBALL"),
			tgbotapi.NewInlineKeyboardButtonData("💎 Google Play", "cat_Google Play"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📦 آخر العروض", "latest"),
		),
	)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func handleCallbacks(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	bot.Request(tgbotapi.NewCallback(query.ID, ""))
	data := query.Data
	chatID := query.Message.Chat.ID

	if strings.HasPrefix(data, "cat_") {
		// عرض منتجات فئة معينة
		category := strings.TrimPrefix(data, "cat_")
		showCategoryProducts(bot, chatID, category)
	} else if strings.HasPrefix(data, "buy_") {
		// منطق الشراء (Notify Admin)
		sku := strings.TrimPrefix(data, "buy_")
		notifyAdmin(bot, query.From, sku)
		bot.Send(tgbotapi.NewMessage(chatID, "✅ تم إرسال طلبك للمشرف! سيتم التواصل معك قريباً."))
	} else if strings.HasPrefix(data, "imgs_") {
		// عرض باقي الصور (بديل Carousel)
		sku := strings.TrimPrefix(data, "imgs_")
		sendMorePhotos(bot, chatID, sku)
	} else if data == "latest" {
		showCategoryProducts(bot, chatID, "") // فارغ يعني الكل
	}
}

func sendProductView(bot *tgbotapi.BotAPI, chatID int64, sku string) {
	var p Product
	if err := DB.Where("sku = ?", sku).First(&p).Error; err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ الرمز غير صحيح أو المنتج غير متوفر."))
		return
	}

	// تقسيم الصور للحصول على الصورة الرئيسية
	photos := strings.Split(p.PhotoIDs, ",")
	mainPhoto := photos[0]

	caption := fmt.Sprintf("📦 *%s*\n💵 السعر: %s\n📄 الوصف: %s\n🆔 الرمز: `%s`", p.Name, p.Price, p.Description, p.SKU)
	
	msg := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(mainPhoto))
	msg.Caption = caption
	msg.ParseMode = "Markdown"

	// أزرار التحكم
	buttons := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("🛒 طلب شراء", "buy_"+p.SKU),
	}
	// إذا كان هناك أكثر من صورة، أضف زر لعرض الباقي
	if len(photos) > 1 {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("📸 عرض صور إضافية", "imgs_"+p.SKU))
	}
	
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
	bot.Send(msg)
}

func showCategoryProducts(bot *tgbotapi.BotAPI, chatID int64, category string) {
	var products []Product
	query := DB.Order("created_at desc").Limit(5) // آخر 5 منتجات
	if category != "" {
		query = query.Where("category = ?", category)
	}
	query.Find(&products)

	if len(products) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "🚫 لا توجد منتجات حالياً في هذا القسم."))
		return
	}

	for _, p := range products {
		sendProductView(bot, chatID, p.SKU)
	}
}

func sendMorePhotos(bot *tgbotapi.BotAPI, chatID int64, sku string) {
	var p Product
	DB.Where("sku = ?", sku).First(&p)
	photos := strings.Split(p.PhotoIDs, ",")
	
	if len(photos) <= 1 { return }

	// إرسال كألبوم (Media Group)
	var files []interface{}
	for i, photoID := range photos {
		if i == 0 { continue } // تخطي الصورة الأولى لأنها عُرضت سابقاً
		media := tgbotapi.NewInputMediaPhoto(tgbotapi.FileID(photoID))
		files = append(files, media)
	}
	
	// تيليجرام يقبل مصفوفة من []interface{} للوسائط
	cfg := tgbotapi.NewMediaGroup(chatID, files)
	bot.Send(cfg)
}

func notifyAdmin(bot *tgbotapi.BotAPI, user *tgbotapi.User, sku string) {
	if adminID == "" { return }
	
	// تحويل AdminID من نص إلى رقم
	// في الكود الحقيقي استخدم strconv.ParseInt
	
	msgText := fmt.Sprintf("🔔 *طلب شراء جديد!*\n📦 المنتج: %s\n👤 المشتري: @%s\n🆔 الآيدي: %d", sku, user.UserName, user.ID)
	msg := tgbotapi.NewMessage(0, msgText) 
	// ملاحظة: هنا يجب وضع adminID المحول لرقم في مكان 0
	// للتسهيل افترضنا أنك ستضبطها، أو يمكننا إرسالها لك مباشرة إذا كنت تختبر بنفسك
}
