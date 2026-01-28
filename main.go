package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// نفس هيكل البيانات الأصلي
type Product struct {
	SKU            string `gorm:"primaryKey"`
	Category       string
	PaymentMethods string
	PriceVal       string
	Description    string
	Mediators      string
	Images         string `gorm:"type:text"`
	SellerLink     string // سيصبح رابط حساب التيليجرام هنا
	IsReserved     bool   `gorm:"default:false"`
	ReservedUntil  time.Time
	CreatedAt      time.Time
}

var DB *gorm.DB
var UserStates = make(map[int64]map[string]string)
var AdminID int64 = 0 // سيتم التعرف عليه عند أول رسالة منك
var FixedMediators = "احمد فرقان / Ayoub wolf / ma ski"

func main() {
	// 1. الاتصال بقاعدة البيانات (نفس الرابط الداخلي في Render)
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("فشل الاتصال بالقاعدة:", err)
	}
	DB.AutoMigrate(&Product{})

	// 2. تشغيل بوت تيليجرام
	botToken := os.Getenv("TELEGRAM_APITOKEN")
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("فشل تشغيل البوت:", err)
	}

	bot.Debug = true
	log.Printf("تم التشغيل على حساب: %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { continue }

		uid := update.Message.Chat.ID
		text := update.Message.Text
		
		// التقاط الصور
		var photoID string
		if update.Message.Photo != nil {
			// نأخذ أعلى جودة للصورة
			photoID = update.Message.Photo[len(update.Message.Photo)-1].FileID
		}

		handleLogic(bot, uid, text, photoID, update.Message.From.UserName)
	}
}

func handleLogic(bot *tgbotapi.BotAPI, uid int64, text string, photoID string, username string) {
	state, exists := UserStates[uid]
	lowerText := strings.ToLower(text)

	// أوامر المدير
	if lowerText == "لوحة التحكم" && uid == AdminID {
		sendMsg(bot, uid, "أهلاً يا شيخ! 👑\n- (حذف SKU) للحذف\n- (حجز SKU) للحجز")
		return
	}

	if !exists || lowerText == "/start" || lowerText == "مرحبا" {
		UserStates[uid] = map[string]string{"step": "CHOOSING", "img_list": ""}
		sendMsg(bot, uid, "مرحبا بك في متجر النخبة (تيليجرام) 🛒\n- أرسل (شراء) للبحث\n- أرسل (بيع) للعرض\n- أرسل (بحث) برمز SKU")
		return
	}

	step := state["step"]

	// نظام الصور (9 صور كحد أقصى)
	if step == "SELL_DESC" {
		if photoID != "" {
			current := state["img_list"]
			count := strings.Count(current, "|")
			if current == "" { count = -1 }
			if count < 8 {
				if current == "" { current = photoID } else { current += "|" + photoID }
				UserStates[uid]["img_list"] = current
				sendMsg(bot, uid, fmt.Sprintf("✅ تم استلام الصورة (%d/9). أرسل المزيد أو (تم).", count+2))
			}
			return
		}
		if lowerText == "تم" {
			UserStates[uid]["step"] = "SELL_MED"
			sendMsg(bot, uid, "اختر الوسطاء:\n"+FixedMediators)
			return
		}
	}

	// منطق البيع والشراء (نفس التدرج)
	if lowerText == "بيع" || strings.HasPrefix(step, "SELL_") {
		handleSale(bot, uid, text, state, username)
	} else if lowerText == "شراء" || strings.HasPrefix(step, "WAIT_") {
		handlePurchase(bot, uid, text, state)
	} else if lowerText == "بحث" || step == "QUICK_SEARCH" {
		handleSearch(bot, uid, text, state)
	}
}

func handleSale(bot *tgbotapi.BotAPI, uid int64, text string, state map[string]string, username string) {
	switch state["step"] {
	case "CHOOSING":
		UserStates[uid]["step"] = "SELL_CAT"
		sendMsg(bot, uid, "ماذا تبيع؟ (فري فاير / ببجي / بيس / خدمة)")
	case "SELL_CAT":
		UserStates[uid]["s_cat"] = text
		UserStates[uid]["step"] = "SELL_PAY"
		sendMsg(bot, uid, "طرق الدفع؟")
	case "SELL_PAY":
		UserStates[uid]["s_pay"] = text
		UserStates[uid]["step"] = "SELL_PRICE"
		sendMsg(bot, uid, "أدخل السعر:")
	case "SELL_PRICE":
		UserStates[uid]["s_price"] = text
		UserStates[uid]["step"] = "SELL_DESC"
		sendMsg(bot, uid, "أرسل الوصف + الصور. أرسل (تم) عند الانتهاء.")
	case "SELL_MED":
		sku := generateSKU()
		sLink := "https://t.me/" + username
		p := Product{
			SKU: sku, Category: state["s_cat"], PaymentMethods: state["s_pay"],
			PriceVal: state["s_price"], Description: state["s_desc"],
			Mediators: text, Images: state["img_list"], SellerLink: sLink,
			CreatedAt: time.Now(),
		}
		DB.Create(&p)
		sendMsg(bot, uid, "✅ تم التسجيل! الرمز: "+sku+"\nرابط حسابك أضيف تلقائياً.")
		UserStates[uid] = map[string]string{"step": "START"}
	}
}

// دوال مساعدة للتيليجرام
func sendMsg(bot *tgbotapi.BotAPI, uid int64, text string) {
	msg := tgbotapi.NewMessage(uid, text)
	bot.Send(msg)
}

func generateSKU() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 4)
	for i := range b { b[i] = chars[r.Intn(len(chars))] }
	return string(b)
}

// ... (تكملة دوال handlePurchase و handleSearch بنفس المنطق)
