package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// --- هيكل قاعدة البيانات ---
type Product struct {
	gorm.Model
	SKU         string `gorm:"uniqueIndex"`
	Category    string
	Price       string
	Payment     string
	Description string
	PhotoIDs    string // لتخزين معرفات الصور مفصولة بفاصلة
	Seller      string
}

var DB *gorm.DB
var userState = make(map[int64]string)
var tempProduct = make(map[int64]*Product)
var photoCounter = make(map[int64]int)

func main() {
	// 1. تشغيل سيرفر الـ Health Check لـ Render
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "System Online")
		})
		log.Printf("HTTP Server starting on port %s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// 2. الاتصال بقاعدة البيانات
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}
	
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("خطأ في الاتصال بالقاعدة:", err)
	}
	
	// التأكد من إنشاء الجداول
	if err := DB.AutoMigrate(&Product{}); err != nil {
		log.Fatal("خطأ في إنشاء الجداول:", err)
	}
	log.Println("Database connected successfully")

	// 3. إعداد البوت
	botToken := os.Getenv("TELEGRAM_APITOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_APITOKEN not set")
	}
	
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic("خطأ في إنشاء البوت:", err)
	}
	
	bot.Debug = false
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	log.Println("Bot is running...")

	for update := range updates {
		if update.Message == nil {
			continue
		}
		
		msg := update.Message
		chatID := msg.Chat.ID
		
		// معالجة الصور أولاً
		if msg.Photo != nil {
			handlePhoto(bot, chatID, msg)
			continue
		}
		
		// معالجة الرسائل النصية
		txt := strings.TrimSpace(msg.Text)
		if txt == "" {
			continue
		}
		
		txtLower := strings.ToLower(txt)

		// --- منطق الترحيب ---
		if isGreeting(txtLower) {
			sendMsg(bot, chatID, "مرحبا 👋\n\nلعرض خدمة أو منتج للبيع أرسل: *بيع*\nللبحث عن منتج معين أرسل: *شراء*")
			continue
		}

		// --- بدء عملية البيع ---
		if txtLower == "بيع" {
			userState[chatID] = "WAIT_CAT"
			userName := msg.From.UserName
			if userName == "" {
				userName = msg.From.FirstName
			}
			tempProduct[chatID] = &Product{Seller: userName}
			photoCounter[chatID] = 0
			sendMsg(bot, chatID, "📦 *خطوة 1/5: الفئة*\n\nيرجى تحديد فئة المنتج:\n• Free Fire\n• PUBG\n• EFOOTBALL\n• Google Play\n• بيع $")
			continue
		}

		// --- بدء عملية الشراء ---
		if txtLower == "شراء" {
			handleSearch(bot, chatID)
			continue
		}

		// --- معالجة خطوات البيع ---
		if state, ok := userState[chatID]; ok {
			handleSteps(bot, chatID, msg, state)
			continue
		}

		// رسالة افتراضية
		sendMsg(bot, chatID, "لم أفهم طلبك 🤔\n\nأرسل *بيع* لعرض منتج\nأو *شراء* للبحث عن منتج")
	}
}

func handlePhoto(bot *tgbotapi.BotAPI, chatID int64, msg *tgbotapi.Message) {
	state, exists := userState[chatID]
	if !exists || state != "WAIT_PHOTOS" {
		sendMsg(bot, chatID, "⚠️ يرجى بدء عملية البيع أولاً بإرسال كلمة: *بيع*")
		return
	}

	p := tempProduct[chatID]
	if p == nil {
		sendMsg(bot, chatID, "⚠️ حدث خطأ. يرجى البدء من جديد بإرسال: *بيع*")
		delete(userState, chatID)
		return
	}

	if photoCounter[chatID] >= 9 {
		sendMsg(bot, chatID, "⚠️ لقد وصلت للحد الأقصى (9 صور).\nأرسل *تم* لإنهاء العملية.")
		return
	}

	// حفظ معرف الصورة
	fileID := msg.Photo[len(msg.Photo)-1].FileID
	if p.PhotoIDs == "" {
		p.PhotoIDs = fileID
	} else {
		p.PhotoIDs += "," + fileID
	}
	photoCounter[chatID]++

	remaining := 9 - photoCounter[chatID]
	if remaining > 0 {
		sendMsg(bot, chatID, fmt.Sprintf("✅ تم استلام الصورة %d/9\n\nيمكنك إرسال %d صورة إضافية أو أرسل *تم* للإنهاء", photoCounter[chatID], remaining))
	} else {
		sendMsg(bot, chatID, "✅ تم استلام 9 صور (الحد الأقصى)\n\nأرسل *تم* للحصول على الرمز")
	}
}

func handleSteps(bot *tgbotapi.BotAPI, chatID int64, msg *tgbotapi.Message, state string) {
	p := tempProduct[chatID]
	if p == nil {
		sendMsg(bot, chatID, "⚠️ حدث خطأ. يرجى البدء من جديد بإرسال: *بيع*")
		delete(userState, chatID)
		return
	}

	txt := strings.TrimSpace(msg.Text)
	if txt == "" {
		sendMsg(bot, chatID, "⚠️ يرجى إرسال نص وليس رسالة فارغة")
		return
	}

	switch state {
	case "WAIT_CAT":
		p.Category = txt
		userState[chatID] = "WAIT_PRICE"
		sendMsg(bot, chatID, "💰 *خطوة 2/5: السعر*\n\nيرجى تحديد السعر (مثال: 500 DA أو 5$)")

	case "WAIT_PRICE":
		p.Price = txt
		userState[chatID] = "WAIT_PAY"
		sendMsg(bot, chatID, "💳 *خطوة 3/5: طريقة الدفع*\n\nيرجى تحديد طرق الدفع المقبولة:\n• بريدي موب\n• CCP\n• $\n• فليكسي\n\n(يمكنك كتابة أكثر من طريقة)")

	case "WAIT_PAY":
		p.Payment = txt
		userState[chatID] = "WAIT_DESC"
		sendMsg(bot, chatID, "📝 *خطوة 4/5: الوصف*\n\nيرجى كتابة وصف مفصل للمنتج ومواصفاته المميزة")

	case "WAIT_DESC":
		p.Description = txt
		userState[chatID] = "WAIT_PHOTOS"
		sendMsg(bot, chatID, "📸 *خطوة 5/5: الصور*\n\nيرجى إرسال صور المنتج (1-9 صور)\n\nبعد الانتهاء أرسل كلمة: *تم*")

	case "WAIT_PHOTOS":
		// معالجة كلمة "تم"
		if strings.ToLower(txt) == "تم" {
			if photoCounter[chatID] > 0 {
				finalizeProduct(bot, chatID)
			} else {
				sendMsg(bot, chatID, "⚠️ يرجى إرسال صورة واحدة على الأقل قبل إرسال *تم*")
			}
			return
		}
		
		sendMsg(bot, chatID, "⚠️ يرجى إرسال صور (وليس نص)\n\nبعد الانتهاء أرسل: *تم*")
	}
}

func finalizeProduct(bot *tgbotapi.BotAPI, chatID int64) {
	p := tempProduct[chatID]
	if p == nil {
		sendMsg(bot, chatID, "⚠️ حدث خطأ")
		return
	}

	// توليد رمز SKU فريد (4 أرقام)
	p.SKU = fmt.Sprintf("%04d", (time.Now().UnixNano()/1000)%10000)

	// محاولة حفظ المنتج
	err := DB.Create(p).Error
	if err != nil {
		log.Printf("Error saving product: %v", err)
		sendMsg(bot, chatID, "⚠️ حدث خطأ في حفظ المنتج. يرجى المحاولة لاحقاً")
		return
	}

	// رسالة النجاح
	msg := fmt.Sprintf("✅ *تم تسجيل منتجك بنجاح!*\n\n"+
		"🔢 *رمز المنتج:* `%s`\n"+
		"📦 *الفئة:* %s\n"+
		"💰 *السعر:* %s\n"+
		"💳 *الدفع:* %s\n"+
		"📸 *عدد الصور:* %d\n\n"+
		"احتفظ بالرمز لمتابعة طلبك 📝",
		p.SKU, p.Category, p.Price, p.Payment, photoCounter[chatID])

	sendMsg(bot, chatID, msg)

	// تنظيف الحالة
	delete(userState, chatID)
	delete(tempProduct, chatID)
	delete(photoCounter, chatID)
}

func handleSearch(bot *tgbotapi.BotAPI, chatID int64) {
	var products []Product
	result := DB.Find(&products)

	if result.Error != nil {
		sendMsg(bot, chatID, "⚠️ حدث خطأ في البحث")
		return
	}

	if len(products) == 0 {
		sendMsg(bot, chatID, "😔 لا توجد منتجات متاحة حالياً")
		return
	}

	msg := "🛍️ *المنتجات المتاحة:*\n\n"
	for i, p := range products {
		if i >= 10 {
			break // عرض أول 10 منتجات فقط
		}
		msg += fmt.Sprintf("━━━━━━━━━━━━━━━\n"+
			"🔢 *الرمز:* `%s`\n"+
			"📦 *الفئة:* %s\n"+
			"💰 *السعر:* %s\n"+
			"💳 *الدفع:* %s\n"+
			"👤 *البائع:* @%s\n\n",
			p.SKU, p.Category, p.Price, p.Payment, p.Seller)
	}

	msg += "للحصول على تفاصيل منتج معين، أرسل: `معلومات الرمز`\nمثال: `معلومات 1234`"
	sendMsg(bot, chatID, msg)
}

func isGreeting(t string) bool {
	greetings := []string{"مرحبا", "مرحبأ", "سلام", "السلام عليكم", "السلام", "وي", "صباح الخير", "مساء الخير", "الخير", "هلا", "اهلا", "هاي", "hi", "hello", "hey"}
	for _, g := range greetings {
		if strings.Contains(t, g) {
			return true
		}
	}
	return false
}

func sendMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}
