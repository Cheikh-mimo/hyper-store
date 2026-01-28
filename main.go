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

type Product struct {
	SKU            string `gorm:"primaryKey"`
	Category       string
	PaymentMethods string
	PriceVal       string
	Description    string
	Mediators      string
	Images         string `gorm:"type:text"`
	SellerLink     string
	IsReserved     bool   `gorm:"default:false"`
	ReservedUntil  time.Time
	CreatedAt      time.Time
}

var DB *gorm.DB
var UserStates = make(map[int64]map[string]string)
var FixedMediators = "احمد فرقان / Ayoub wolf / ma ski"

func main() {
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err == nil { DB.AutoMigrate(&Product{}) }

	botToken := os.Getenv("TELEGRAM_APITOKEN")
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil { log.Panic(err) }

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { continue }
		uid := update.Message.Chat.ID
		text := update.Message.Text
		var photoID string
		if update.Message.Photo != nil {
			photoID = update.Message.Photo[len(update.Message.Photo)-1].FileID
		}
		logicHandler(bot, uid, text, photoID, update.Message.From.UserName)
	}
}

func logicHandler(bot *tgbotapi.BotAPI, uid int64, text string, photoID string, username string) {
	state, exists := UserStates[uid]
	lowerText := strings.ToLower(text)

	if isAdmin(uid) && handleAdminCommands(bot, uid, lowerText) { return }

	if strings.HasPrefix(lowerText, "حجز ") {
		sku := strings.ToUpper(strings.TrimPrefix(lowerText, "حجز "))
		reserveProduct(bot, uid, sku)
		return
	}

	if !exists || lowerText == "/start" || lowerText == "مرحبا" {
		UserStates[uid] = map[string]string{"step": "CHOOSING", "img_list": ""}
		sendMsg(bot, uid, "مرحبا بك في متجر النخبة 🛒\n- (شراء) للبحث\n- (بيع) للعرض\n- (بحث) برمز SKU\n- (حجز SKU) للحجز")
		return
	}

	step := state["step"]
	if step == "SELL_DESC" {
		if photoID != "" {
			current := state["img_list"]
			if strings.Count(current, "|") < 8 {
				if current == "" { current = photoID } else { current += "|" + photoID }
				UserStates[uid]["img_list"] = current
				sendMsg(bot, uid, "✅ تم استلام الصورة. أرسل المزيد أو (تم).")
			}
			return
		}
		if lowerText == "تم" {
			UserStates[uid]["step"] = "SELL_MED"
			sendMsg(bot, uid, "اختر الوسطاء:\n"+FixedMediators)
			return
		}
	}

	if lowerText == "بيع" || strings.HasPrefix(step, "SELL_") {
		handleSale(bot, uid, text, state, username)
	} else if lowerText == "شراء" || strings.HasPrefix(step, "WAIT_") {
		handlePurchase(bot, uid, text, state)
	} else if lowerText == "بحث" || step == "QUICK_SEARCH" {
		handleQuickSearch(bot, uid, text)
	}
}

func handleSale(bot *tgbotapi.BotAPI, uid int64, text string, state map[string]string, username string) {
	switch state["step"] {
	case "CHOOSING":
		UserStates[uid]["step"] = "SELL_CAT"
		sendMsg(bot, uid, "ماذا تبيع؟ (فري فاير/ببجي/بيس/خدمة)")
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
		p := Product{SKU: sku, Category: state["s_cat"], PaymentMethods: state["s_pay"], PriceVal: state["s_price"], Description: state["s_desc"], Mediators: text, Images: state["img_list"], SellerLink: sLink, CreatedAt: time.Now()}
		DB.Create(&p)
		sendMsg(bot, uid, "✅ تم التسجيل! الرمز: "+sku)
		UserStates[uid] = map[string]string{"step": "START"}
	}
}

func handlePurchase(bot *tgbotapi.BotAPI, uid int64, text string, state map[string]string) {
	switch state["step"] {
	case "CHOOSING":
		UserStates[uid]["step"] = "WAIT_CAT"
		sendMsg(bot, uid, "ما الصنف المطلوب؟")
	case "WAIT_CAT":
		UserStates[uid]["cat"] = text
		var products []Product
		DB.Where("category ILIKE ? AND is_reserved = ?", "%"+text+"%", false).Limit(5).Find(&products)
		if len(products) == 0 {
			sendMsg(bot, uid, "❌ لا توجد نتائج حالياً.")
		} else {
			for _, p := range products {
				res := fmt.Sprintf("📦 الرمز: %s\n💰 السعر: %s\n📝 الوصف: %s\n👤 بائع: %s", p.SKU, p.PriceVal, p.Description, p.SellerLink)
				sendMsg(bot, uid, res)
			}
		}
		UserStates[uid] = map[string]string{"step": "START"}
	}
}

func handleQuickSearch(bot *tgbotapi.BotAPI, uid int64, text string) {
	if strings.ToLower(text) == "بحث" {
		UserStates[uid]["step"] = "QUICK_SEARCH"
		sendMsg(bot, uid, "أدخل رمز SKU:")
		return
	}
	var p Product
	if DB.First(&p, "sku = ?", strings.ToUpper(text)).Error == nil {
		res := fmt.Sprintf("🔍 تفاصيل %s:\n💰 السعر: %s\n📝 الوصف: %s\n👤 بائع: %s", p.SKU, p.PriceVal, p.Description, p.SellerLink)
		sendMsg(bot, uid, res)
		if p.Images != "" {
			for _, img := range strings.Split(p.Images, "|") {
				bot.Send(tgbotapi.NewPhoto(uid, tgbotapi.FileID(img)))
			}
		}
	} else { sendMsg(bot, uid, "❌ رمز غير موجود.") }
	UserStates[uid] = map[string]string{"step": "START"}
}

func reserveProduct(bot *tgbotapi.BotAPI, uid int64, sku string) {
	var p Product
	if DB.First(&p, "sku = ?", sku).Error == nil {
		p.IsReserved = true
		p.ReservedUntil = time.Now().Add(24 * time.Hour)
		DB.Save(&p)
		sendMsg(bot, uid, "✅ تم الحجز لـ 24 ساعة.")
	} else { sendMsg(bot, uid, "❌ الرمز خاطئ.") }
}

func sendMsg(bot *tgbotapi.BotAPI, uid int64, text string) {
	bot.Send(tgbotapi.NewMessage(uid, text))
}

func generateSKU() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 4)
	for i := range b { b[i] = chars[r.Intn(len(chars))] }
	return string(b)
}
