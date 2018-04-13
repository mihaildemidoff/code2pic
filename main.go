package main

import (
	"net/http"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"sourcegraph.com/sourcegraph/go-selenium"
	"fmt"
	"bytes"
	"strings"
	"strconv"
	"time"
	"html/template"
)

type IncomingMessage struct {
	ID      int
	ChatId  int64
	Message string
}

type PreparedMessage struct {
	IncomingMessage
	ErrorText string
}

var Template, _ = template.ParseFiles("index.html")

func main() {
	conf, err := loadConfig()
	checkFatalError(err, "Couldn't load config ")
	cache := new(RedisCache)
	err = cache.Connect(conf.RedisSettings)
	checkFatalError(err, "Couldn't connect to redis ")
	defer cache.Close()
	tgBot, err := createTgConnection(&conf.TelegramSettings)
	checkFatalError(err, "Couldn't connect to telegram ")

	taskChan := make(chan *IncomingMessage, 100)
	responseChan := make(chan *PreparedMessage, 100)
	go listenMessage(cache, tgBot, taskChan)
	go sendChannelListener(cache, tgBot, responseChan)
	for i := 0; i < conf.NumberOfGeneratorWorkers; i++ {
		go generator(cache, taskChan, responseChan)
	}
	startHttpServer(cache)
}

func checkFatalError(err error, message string) {
	if err != nil {
		panic(message + err.Error())
	}
}

func createTgConnection(config *TelegramSettings) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(config.Secret)
	if err != nil {
		return nil, err
	}
	bot.Debug = config.Debug
	log.Printf("Authorized on account %s", bot.Self.UserName)
	return bot, nil
}

func sendChannelListener(cache FileCache, bot *tgbotapi.BotAPI, responseChan chan *PreparedMessage) {
	// Naive throttling
	ticker := time.NewTicker(time.Nanosecond)
	for t := range ticker.C {
		_ = t
		select {
		case response := <-responseChan:
			go func() {
				if len(response.ErrorText) <= 0 {
					fileBytes, err := cache.GetBytes(strconv.Itoa(response.ID))
					if err != nil {
						log.Println(err.Error())
					}
					file := tgbotapi.FileBytes{Name: "Image", Bytes: fileBytes}
					msg := tgbotapi.NewPhotoUpload(response.ChatId, file)
					bot.Send(msg)
				} else {
					msg := tgbotapi.NewMessage(response.ChatId, response.ErrorText)
					bot.Send(msg)
				}
			}()
		default:

		}
	}

}

// Listens incoming message from telegram. If message is not empty than incoming text is added to cache.
// Incoming messages are non blocking
func listenMessage(cache FileCache, bot *tgbotapi.BotAPI, taskChan chan *IncomingMessage) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message != nil || len(update.Message.Text) != 0 {
			go func() {
				log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
				message := IncomingMessage{ChatId: update.Message.Chat.ID, Message: update.Message.Text, ID: update.UpdateID}
				cache.SaveText(strconv.Itoa(message.ID), update.Message.Text)
				taskChan <- &message
			}()
		}
	}
}

func startHttpServer(cache FileCache) {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		id := request.URL.Path
		id = strings.TrimPrefix(id, "/")
		code, _ := cache.GetText(id)
		Template.Execute(writer, code)
	})
	if err := http.ListenAndServe(":8085", nil); err != nil {
		panic(err)
	}
}

func generator(cache FileCache, taskChan chan *IncomingMessage, responseChan chan *PreparedMessage) {
	for message := range taskChan {
		file, err := generateImage(message.ID)
		var response *PreparedMessage
		if err != nil {
			response = &PreparedMessage{ErrorText: "Error occured", IncomingMessage: IncomingMessage{ChatId: message.ChatId, Message: message.Message,
				ID: message.ID}}
		} else {
			err = cache.SaveBytes(strconv.Itoa(message.ID), file)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			response = &PreparedMessage{ErrorText: "", IncomingMessage: IncomingMessage{ChatId: message.ChatId, Message: message.Message,
				ID: message.ID}}
			responseChan <- response
		}

	}
}

func generateImage(ID int) ([]byte, error) {
	var webDriver selenium.WebDriver
	var err error
	caps := selenium.Capabilities(map[string]interface{}{"browserName": "chrome"})
	if webDriver, err = selenium.NewRemote(caps, "http://localhost:8910"); err != nil {
		fmt.Printf("Failed to open session: %s\n", err)
		return nil, err
	}
	defer webDriver.Quit()
	err = webDriver.Get("http://192.168.1.35:8085/" + strconv.Itoa(ID))
	if err != nil {
		fmt.Printf("Failed to load page: %s\n", err)
		return nil, err
	}

	window, err := webDriver.CurrentWindowHandle()
	webDriver.ResizeWindow(window, selenium.Size{Width: 1000, Height: 1000})

	reader, err := webDriver.Screenshot()
	if err != nil {
		fmt.Printf("Failed to load screenshot: %s\n", err)
		return nil, err
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	return buf.Bytes(), nil
}
