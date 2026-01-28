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
	SKU          string `gorm:"uniqueIndex"`
	Category     string
	Price        string
	Payment      string
	Description  string
	PhotoIDs     string // لتخزين روابط الصور مفصولة بفاصلة
	Seller       string
}

var DB *gorm.DB
var userState = make(map[int64]string)
var tempProduct = make(map[int64]*Product)
var photoCounter = make(map[int64]int)

func main() {
	// 1. تشغيل سيرفر الـ Health Check لـ Render
	go func() {
		port := os.Getenv("PORT")
		if port == "" { port = "8080" }
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "System Online") })
		http.ListenAndServe(":"+port, nil)
	}()

	// 2. الاتصال بقاعدة البيانات
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { log.Fatal("خطأ في الاتصال بالقاعدة:", err) }
	DB.AutoMigrate(&Product{})

	// 3. إعداد البوت
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil { log.Panic(err) }

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { continue }
		msg := update.Message
		chatID := msg.Chat.ID
		txt := strings.TrimSpace(msg.Text)
		txtLower := strings.ToLower(txt)

		// --- منطق الترحيب (A: مرحبا -> B: الرد المخصص) ---
		if isGreeting(txtLower) {
			sendMsg(bot, chatID, "مرحبا . لعرض خدمة او منتج للبيع ارسل (بيع) وان اردت البحث عن منتج معين ارسل (شراء)")
			continue
		}

		// --- بدء عملية البيع ---
		if txtLower == "بيع" {
			userState[chatID] = "WAIT_CAT"
			tempProduct[chatID] = &Product{Seller: msg.From.UserName}
			sendMsg(bot, chatID, "يرجى تحديد الفئة الخاصة بالمنتج : Free Fire / PUBG / EFOOTBALL / Google Play / بيع $")
			continue
		}

		// --- معالجة خطوات البيع ---
		if state, ok := userState[chatID]; ok {
			handleSteps(bot, chatID, msg, state)
			continue
		}
	}
}

func handleSteps(bot *tgbotapi.BotAPI, chatID int64, msg *tgbotapi.Message, state string) {
	p := tempProduct[chatID]
	txt := msg.Text

	switch state {
	case "WAIT_CAT":
		p.Category = txt
		userState[chatID] = "WAIT_PRICE"
		sendMsg(bot, chatID, "ممتاز ، الان يرجى تحديد السعر بالعملتين DA أو $")

	case "WAIT_PRICE":
		p.Price = txt
		userState[chatID] = "WAIT_PAY"
		sendMsg(bot, chatID, "يرجى تحديد طريقة/طرق الدفع التي تقبلها : بريدي موب / CCP /$/ فليكسي")

	case "WAIT_PAY":
		p.Payment = txt
		userState[chatID] = "WAIT_DESC"
		sendMsg(bot, chatID, "يرجى تحديد المواصفات المميزة في الحساب")

	case "WAIT_DESC":
		p.Description = txt
		userState[chatID] = "WAIT_PHOTOS"
		photoCounter[chatID] = 0
		sendMsg(bot, chatID, "يرجى ارسال صورة او صور الخاصة بهاذا الحساب (9 صور كحد اقصى)")

	case "WAIT_PHOTOS":
		// إذا كتب المستخدم "تم" ننهي العملية ونعطيه الرمز
		if strings.ToLower(txt) == "تم" {
			if photoCounter[chatID] > 0 {
				finalizeProduct(bot, chatID)
			} else {
				sendMsg(bot, chatID, "يرجى إرسال صورة واحدة على الأقل أولاً.")
			}
			return
		}

		// استقبال الصور
		if msg.Photo != nil {
			if photoCounter[chatID] < 9 {
				fileID := msg.Photo[len(msg.Photo)-1].FileID
				p.PhotoIDs += fileID + ","
				photoCounter[chatID]++
				
				if photoCounter[chatID] == 1 {
					sendMsg(bot, chatID, "تم استلام الصورة. يمكنك إرسال المزيد (حتى 9) ثم أرسل كلمة (تم) للحصول على الرمز.")
				} else if photoCounter[chatID] == 9 {
					sendMsg(bot, chatID, "تم استلام 9 صور. جاري استخراج الرمز...")
					finalizeProduct(bot, chatID)
				}
			}
		}
	}
}

func finalizeProduct(bot *tgbotapi.BotAPI, chatID int64) {
	p := tempProduct[chatID]
	// توليد رمز SKU من 4 أرقام
	p.SKU = fmt.Sprintf("%04d", time.Now().UnixNano()%10000)
	
	if err := DB.Create(p).Error; err == nil {
		sendMsg(bot, chatID, "تم استلام طلبك ، سيتم منحك رمز خاص بطلبك هاذا للبيع ، شكرا")
		time.Sleep(1 * time.Second)
		sendMsg(bot, chatID, "تم تسجيل طلبك : رمز طلبك هو "+p.SKU)
	}

	// تنظيف الحالة
	delete(userState, chatID)
	delete(tempProduct, chatID)
	delete(photoCounter, chatID)
}

func isGreeting(t string) bool {
	greetings := []string{"مرحبا", "سلام", "السلام عليكم", "وي", "صباح الخير", "الخير"}
	for _, g := range greetings {
		if strings.Contains(t, g) { return true }
	}
	return false
}

func sendMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}
