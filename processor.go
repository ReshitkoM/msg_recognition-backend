package main

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
	"log"

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
	if !s.Success {
		text = "An error occured. Try again later."
	} else {
		text = s.Text
	}
	return chatID, text
}

func (proc *processor) stop() {
	proc.mq_connection.Close()
	proc.mq_channel.Close()
	// proc.mq_ctxCancel()
}

func newProcessor(mqConnectString string) *processor {
	//TODO process errors
	
	var conn *amqp.Connection;
	for i := 0; i < 10; i++ {
		conn0, err := amqp.Dial(mqConnectString)
		if err != nil {
			if i == 9 {
				failOnError(err, "Failed to connect to RabbitMQ")
			}
			log.Println("Failed to connect to mq. Retrying...")
			time.Sleep(5 * time.Second)
		} else {
			conn = conn0
			break
		}
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
	}
	failOnError(err, "Failed to open a channel")

	q, err := ch.QueueDeclare(
		"hello", // name
		false,   // durable
		false,   // delete when unused
		true,    // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
	}
	failOnError(err, "Failed to declare a queue")
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		// cancelmq()
		ch.Close()
		conn.Close()
	}
	failOnError(err, "Failed to register a consumer")
	// defer cancelmq()
	return &processor{conn, &q, ch, msgs}
}
