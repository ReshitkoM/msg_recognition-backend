package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io/ioutil"

	"os"
	"strconv"
	"time"

	// tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type appConfig struct {
	Bot botConfig `yaml:"bot"`
}

type botConfig struct {
	Token string `yaml:"token"`
}

type App struct {
	config      *appConfig
	tg_bot      *tgbotapi.BotAPI
	mq_context  *context.Context
	mq_queue    *amqp.Queue
	mq_channel  *amqp.Channel
	mq_consumer <-chan amqp.Delivery
}

type SendMsg struct {
	Text string `json:"text"`
}

func (app *App) StartBot() {
	var err error
	bot, err := tgbotapi.NewBotAPI(app.config.Bot.Token)
	if err != nil {
		// Abort if something is wrong
		log.Panic(err)
	}

	// Set this to true to log all interactions with telegram servers
	bot.Debug = true
	app.tg_bot = bot
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Create a new cancellable background context. Calling `cancel()` leads to the cancellation of the context
	ctx0 := context.Background()
	ctx0, cancel := context.WithCancel(ctx0)

	// `updates` is a golang channel which receives telegram updates
	updates := bot.GetUpdatesChan(u)

	//MQ
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()
	app.mq_channel = ch

	q, err := ch.QueueDeclare(
		"hello", // name
		false,   // durable
		false,   // delete when unused
		true,    // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")
	app.mq_queue = &q
	ctx, cancelmq := context.WithTimeout(context.Background(), 5*time.Second)
	app.mq_context = &ctx

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")
	app.mq_consumer = msgs
	defer cancelmq()

	// Pass cancellable context to goroutine
	go app.receiveUpdates(ctx0, updates)
	// Tell the user the bot is online
	log.Println("Start listening for updates. Press enter to stop")

	// Wait for a newline symbol, then cancel handling updates
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	cancel()

}

// tg bot
func (app *App) getTGFile(fileID string) ([]byte){
	//TODO dont fail, just return error
	url, err := app.tg_bot.GetFileDirectURL(fileID)
	failOnError(err, "Failed to get file url")
	resp, err := http.Get(url)
	failOnError(err, "Failed to get the file")
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	failOnError(err, "Failed to read the file")
	return bytes
}

func (app *App) sendVoiceToTextReq(voice []byte, corrId string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//
	var err = app.mq_channel.PublishWithContext(ctx,
		"",          // exchange
		"rpc_queue", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId,
			ReplyTo:       app.mq_queue.Name,
			Body:          voice,
		})
	failOnError(err, "Failed to publish a message")
}

func (app *App) processReqResult(msg amqp.Delivery) (chatID int64, text string) {
	chatID, err := strconv.ParseInt(msg.CorrelationId, 10, 64)
	if err != nil {
		failOnError(err, "Failed to parse user id")
	}
	var s SendMsg
	err = json.Unmarshal(msg.Body, &s)
	failOnError(err, "Failed to decode msg")
	text = s.Text
	return chatID, text
}

func (app *App) receiveUpdates(ctx context.Context, updates tgbotapi.UpdatesChannel) {
	// `for {` means the loop is infinite until we manually stop it
	for {
		select {
		// stop looping if ctx is cancelled
		case <-ctx.Done():
			return
		// receive update from channel and then handle it
		case update := <-updates:
			log.Println(update.Message.Text)
			//
			if update.Message.Voice != nil {
				var voiceBytes = app.getTGFile(update.Message.Voice.FileID)
				log.Println("Received " + fmt.Sprint(binary.Size(voiceBytes)) + "bytes.")

				app.sendVoiceToTextReq(voiceBytes, strconv.FormatInt(update.Message.Chat.ID, 10))

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "your request is being processed")
				var _, err = app.tg_bot.Send(msg)
				failOnError(err, "Failed to send msg to user")
			} else {

			}
		case d := <- app.mq_consumer:
			chatID, text := app.processReqResult(d)

			msg := tgbotapi.NewMessage(chatID, text)
			_, err := app.tg_bot.Send(msg)
			failOnError(err, "Failed to send msg to user")
		}
	}
}

// config
func readConf(filename string) (*appConfig, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c := &appConfig{}
	err = yaml.Unmarshal(buf, c)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filename, err)
	}

	return c, err
}

// rabbitmq
func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	config, err := readConf("config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// a := App{config}
	a := App{config, nil, nil, nil, nil, nil}
	a.StartBot()
}
