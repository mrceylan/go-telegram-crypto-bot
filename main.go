package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/patrickmn/go-cache"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type cryptoData struct {
	Data    data
	Price   price
	Convert string
}

type jsonResult struct {
	Status status          `json:"status"`
	Data   map[string]data `json:"data"`
}

type status struct {
	ErrorCode    int `json:"error_code"`
	ErrorMessage int `json:"error_message"`
}

type data struct {
	Name              string           `json:"name"`
	Symbol            string           `json:"symbol"`
	MaxSupply         float64          `json:"max_supply"`
	CirculatingSupply float64          `json:"circulating_supply"`
	TotalSupply       float64          `json:"total_supply"`
	Quote             map[string]price `json:"quote"`
}

type price struct {
	Price       float64   `json:"price"`
	Volume      float64   `json:"volume_24h"`
	ChangeHour  float64   `json:"percent_change_1h"`
	ChangeDay   float64   `json:"percent_change_24h"`
	MarketCap   float64   `json:"market_cap"`
	LastUpdated time.Time `json:"last_updated"`
}

var cac *cache.Cache

var botWrongCommandMessage string = "You have entered wrong commmand. Please enter command like BTC-USD."

func main() {
	cac = cache.New(30*time.Second, 10*time.Minute)

	token := os.Getenv("CRYPTO_API_TOKEN")

	if len(token) == 0 {
		panic("No token defined")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	listenCommands(updates, bot)
}

func listenCommands(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI) {
	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message == nil {
			continue
		}

		command := strings.Split(strings.ToUpper(update.Message.Text), "-")

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		if len(command) != 2 {
			msg.Text = botWrongCommandMessage
			bot.Send(msg)
			continue
		}

		result, err := getData(command[0], command[1])
		if err != nil {
			msg.Text = botWrongCommandMessage
			bot.Send(msg)
			continue
		}
		msg.ParseMode = "html"
		msg.Text = generateMessage(result)
		bot.Send(msg)
	}
}

func getData(symbol string, convert string) (cryptoData, error) {

	cacheData, found := cac.Get(symbol + convert)
	if found {
		return cacheData.(cryptoData), nil
	}

	jsonData, err := createRequest(symbol, convert)
	if err != nil {
		return cryptoData{}, err
	}

	data, err := parseJsonToData(jsonData, symbol, convert)
	if err != nil {
		return cryptoData{}, err
	}

	cac.Add(symbol+convert, data, cache.DefaultExpiration)

	return data, nil
}

func parseJsonToData(jsonData []byte, symbol string, convert string) (cryptoData, error) {
	var result jsonResult
	err := json.Unmarshal(jsonData, &result)
	if err != nil {
		return cryptoData{}, err
	}

	_data := result.Data[symbol]
	_price := _data.Quote[convert]

	return cryptoData{_data, _price, convert}, nil
}

func createRequest(symbol string, convert string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://pro-api.coinmarketcap.com/v1/cryptocurrency/quotes/latest", nil)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Add("symbol", symbol)
	q.Add("convert", convert)

	req.Header.Set("Accepts", "application/json")
	req.Header.Add("X-CMC_PRO_API_KEY", "ef8bdafc-a92d-4fe9-8cdd-ebb2e90dda6f")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	return respBody, nil
}

func generateMessage(data cryptoData) string {
	var sb strings.Builder
	printer := message.NewPrinter(language.English)

	printer.Fprintf(&sb, "<b>%s - %s</b>\n", data.Data.Symbol, data.Convert)

	printer.Fprintf(&sb, "\n")

	printer.Fprintf(&sb, "<b>Max Supply: </b><i>%.0f</i> %s\n", data.Data.MaxSupply, data.Data.Symbol)
	printer.Fprintf(&sb, "<b>Circulating Supply: </b><i>%.0f</i> %s\n", data.Data.CirculatingSupply, data.Data.Symbol)
	printer.Fprintf(&sb, "<b>Total Supply: </b><i>%.0f</i> %s\n", data.Data.TotalSupply, data.Data.Symbol)

	printer.Fprintf(&sb, "\n")

	printer.Fprintf(&sb, "<b>Price: </b><i>%.8f</i> %s\n", data.Price.Price, data.Convert)
	printer.Fprintf(&sb, "<b>Volume: </b><i>%.3f</i> %s\n", data.Price.Volume, data.Convert)
	printer.Fprintf(&sb, "<b>1 Hour Change Percent: </b><i>%.2f%%</i>\n", data.Price.ChangeHour)
	printer.Fprintf(&sb, "<b>Daily Change Percent: </b><i>%.2f%%</i>\n", data.Price.ChangeDay)
	printer.Fprintf(&sb, "<b>Market Cap: </b><i>%.3f</i> %s\n", data.Price.MarketCap, data.Convert)
	printer.Fprintf(&sb, "<b>Data Last Updated: </b><i>%v</i>\n", data.Price.LastUpdated.Format("15:04:05"))

	return sb.String()
}
