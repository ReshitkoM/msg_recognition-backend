package main

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type processor struct {
	mq_connection *amqp.Connection
	mq_queue      *amqp.Queue
	mq_channel    *amqp.Channel
	mq_consumer   <-chan amqp.Delivery
	// mq_ctxCancel  context.CancelFunc
}

func (proc *processor) sendVoiceToTextReq(msg []byte, chatId int64) {
	corrId := strconv.FormatInt(chatId, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//
	var err = proc.mq_channel.PublishWithContext(ctx,
		"",          // exchange
		"rpc_queue", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId,
			ReplyTo:       proc.mq_queue.Name,
			Body:          msg,
		})
	failOnError(err, "Failed to publish a message")
}

func (proc *processor) processReqResult(msg amqp.Delivery) (chatID int64, text string) {
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

func (proc *processor) stop() {
	proc.mq_connection.Close()
	proc.mq_channel.Close()
	// proc.mq_ctxCancel()
}

func newProcessor() *processor {
	//TODO process errors
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	// defer conn.Close()

	ch, err := conn.Channel()
	if (err != nil) {
		conn.Close()
	}
	failOnError(err, "Failed to open a channel")
	// defer ch.Close()

	q, err := ch.QueueDeclare(
		"hello", // name
		false,   // durable
		false,   // delete when unused
		true,    // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if (err != nil) {
		ch.Close()
		conn.Close()
	}
	failOnError(err, "Failed to declare a queue")
	// app.mq_queue = &q
	// _, cancelmq := context.WithTimeout(context.Background(), 5*time.Second)
	// app.mq_context = &ctx

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if (err != nil) {
		// cancelmq()
		ch.Close()
		conn.Close()
	}
	failOnError(err, "Failed to register a consumer")
	// defer cancelmq()
	return &processor{conn, &q, ch, msgs}
}
