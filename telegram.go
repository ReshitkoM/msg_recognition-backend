package main

import (
	"context"
	"net/http"
	"io"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)
type telegram struct {
	TelegramMessageChannel 	tgbotapi.UpdatesChannel
	TelegramContext 		context.Context

	bot      				*tgbotapi.BotAPI
	cancel					context.CancelFunc
}

func newTelegram(botToken string) *telegram {
	bot, err := tgbotapi.NewBotAPI(botToken)
	failOnError(err, "Cannot create tg bot")
	bot.Debug = true
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Create a new cancellable background context. Calling `cancel()` leads to the cancellation of the context
	ctx0 := context.Background()
	ctx0, cancel := context.WithCancel(ctx0)

	// `updates` is a golang channel which receives telegram updates
	updates := bot.GetUpdatesChan(u)
	return &telegram{updates, ctx0, bot, cancel}
}


func (tg *telegram) getFile(fileID string) ([]byte){
	//TODO dont fail, just return error
	url, err := tg.bot.GetFileDirectURL(fileID)
	failOnError(err, "Failed to get file url")
	resp, err := http.Get(url)
	failOnError(err, "Failed to get the file")
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	failOnError(err, "Failed to read the file")
	return bytes
}

func (tg *telegram) send(text string, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := tg.bot.Send(msg)
	failOnError(err, "Failed to send msg to user")
}


func (tg *telegram) stop() {
	tg.cancel()
}