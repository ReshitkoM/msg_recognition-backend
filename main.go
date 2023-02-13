package main

import (
	"bufio"
	// "context"
	"fmt"
	"io/ioutil"

	// "log"
	"os"
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
	mq_consumer *<-chan amqp.Delivery
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
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// `updates` is a golang channel which receives telegram updates
	updates := bot.GetUpdatesChan(u)

	// Pass cancellable context to goroutine
	go app.receiveUpdates(ctx, updates)

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
	app.mq_consumer = &msgs
	defer cancelmq()

	// Tell the user the bot is online
	log.Println("Start listening for updates. Press enter to stop")

	// Wait for a newline symbol, then cancel handling updates
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	cancel()

}

// tg bot
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
				//tmp write voice into file
				//file, err := app.tg_bot.GetFile(tgbotapi.FileConfig{update.Message.Voice.FileID})
				file2, _ := app.tg_bot.GetFileDirectURL(update.Message.Voice.FileID)
				if file2 != "" {
					// out, err := os.Create("test.ogg")
					// if err != nil {
					// 	return
					// }
					// defer out.Close()

					resp, err := http.Get(file2)
					if err != nil {
						return
					}
					defer resp.Body.Close()

					b, err := io.ReadAll(resp.Body)
					if err != nil {
						return
					}
					b2 := string(b)

					// Writer the body to file
					// _, err = io.Copy(out, resp.Body)
					// if err != nil {
					// 	return
					// }

					//
					corrId := "42"

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					//
					body := b2
					err = app.mq_channel.PublishWithContext(ctx,
						"",          // exchange
						"rpc_queue", // routing key
						false,       // mandatory
						false,       // immediate
						amqp.Publishing{
							ContentType:   "text/plain",
							CorrelationId: corrId,
							ReplyTo:       app.mq_queue.Name,
							Body:          []byte(body),
						})
					failOnError(err, "Failed to publish a message")
					log.Printf(" [x] Sent %s\n", body)
					for d := range *app.mq_consumer {
						if corrId == d.CorrelationId {
							log.Printf(" [x] Received %s\n", string(d.Body))
							failOnError(err, "Failed to convert body to integer")

							msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
							var s SendMsg
							err = json.Unmarshal(d.Body, &s)
							failOnError(err, "Failed to decode msg")
							msg.Text = s.Text

							_, err := app.tg_bot.Send(msg)
							failOnError(err, "Failed to send msg to user")
							break
						}
					}
				}
			} else {
				body := update.Message.Text
				err := app.mq_channel.PublishWithContext(ctx,
					"",                // exchange
					app.mq_queue.Name, // routing key
					false,             // mandatory
					false,             // immediate
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        []byte(body),
					})
				failOnError(err, "Failed to publish a message")
				log.Printf(" [x] Sent %s\n", body)
			}

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
