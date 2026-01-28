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

// --- الجداول ---
type Product struct {
	gorm.Model
	SKU          string `gorm:"uniqueIndex"`
	Category     string
	Price        string
	Payment      string
	Description  string
	PhotoIDs     string // سنخزن معرفات الصور مفصولة بفاصلة
	Seller       string
	Reserved     bool
}

var DB *gorm.DB
var userState = make(map[int64]string)
var tempProduct = make(map[int64]*Product)
var photoCounter = make(map[int64]int)

func main() {
	// 1. نظام الحماية من Render (Port)
	go func() {
		port := os.Getenv("PORT")
		if port == "" { port = "8080" }
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "Bot Active") })
		http.ListenAndServe(":"+port, nil)
	}()

	// 2. قاعدة البيانات
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { log.Fatal(err) }
	DB.AutoMigrate(&Product{})

	// 3. تشغيل البوت
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil { log.Panic(err) }

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { continue }
		msg := update.Message
		chatID := msg.Chat.ID
		txt := strings.ToLower(msg.Text)

		// --- منطق الترحيب ---
		if txt == "مرحبا" || txt == "سلام" || txt == "السلام عليكم" || txt == "وي" || strings.Contains(txt, "الله") {
			sendMsg(bot, chatID, "مرحبا . لعرض خدمة او منتج للبيع ارسل (بيع) وان اردت البحث عن منتج معين ارسل (شراء)")
			continue
		}

		// --- بدء عملية البيع ---
		if txt == "بيع" {
			userState[chatID] = "WAIT_CAT"
			tempProduct[chatID] = &Product{Seller: msg.From.UserName}
			sendMsg(bot, chatID, "يرجى تحديد الفئة الخاصة بالمنتج : Free Fire / PUBG / EFOOTBALL / Google Play / بيع $")
			continue
		}

		// --- معالجة التدفق المتسلسل ---
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
		if msg.Photo != nil {
			count := photoCounter[chatID]
			if count < 9 {
				fileID := msg.Photo[len(msg.Photo)-1].FileID
				p.PhotoIDs += fileID + ","
				photoCounter[chatID]++
				
				if photoCounter[chatID] == 1 {
					sendMsg(bot, chatID, "تم استلام أول صورة، يمكنك إرسال المزيد حتى 9 أو انتظر التسجيل...")
					// مؤقت بسيط لإنهاء الاستلام إذا توقف المستخدم عن الإرسال
					go func(cID int64) {
						time.Sleep(10 * time.Second)
						finalizeProduct(bot, cID)
					}(chatID)
				}
			}
		}
	}
}

func finalizeProduct(bot *tgbotapi.BotAPI, chatID int64) {
	if _, ok := userState[chatID]; !ok || userState[chatID] != "WAIT_PHOTOS" { return }
	
	p := tempProduct[chatID]
	p.SKU = fmt.Sprintf("%04d", time.Now().UnixNano()%10000)
	
	if err := DB.Create(p).Error; err == nil {
		sendMsg(bot, chatID, "تم تسجيل طلبك : رمز طلبك هو " + p.SKU)
	}
	delete(userState, chatID)
	delete(tempProduct, chatID)
	delete(photoCounter, chatID)
}

func sendMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}
