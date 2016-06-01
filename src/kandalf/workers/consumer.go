package workers

import (
	"sync"

	"github.com/streadway/amqp"

	"../logger"
)

var (
	exchangeName string = "amq.rabbitmq.trace"
	routingKey   string = "publish.*"
)

type internalConsumer struct {
	con   *amqp.Connection
	queue *internalQueue
}

// Returns new instance of RabbitMQ consumer
func newInternalConsumer(url string, queue *internalQueue) (*internalConsumer, error) {
	con, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	return &internalConsumer{
		con:   con,
		queue: queue,
	}, nil
}

// Main working cycle
func (c *internalConsumer) run(wg *sync.WaitGroup, die chan bool) {
	defer wg.Done()

	ch, err := c.con.Channel()
	if err != nil {
		logger.Instance().
			WithError(err).
			Warning("Unable to open channel")

		return
	}

	err = ch.ExchangeDeclare(exchangeName, "topic", true, true, true, false, nil)
	if err != nil {
		logger.Instance().
			WithError(err).
			Warning("Unable to declare exchange")

		return
	}

	q, err := ch.QueueDeclare("", false, true, false, true, nil)
	if err != nil {
		logger.Instance().
			WithError(err).
			Warning("Unable to declare queue")

		return
	}

	err = ch.QueueBind(q.Name, routingKey, exchangeName, true, nil)
	if err != nil {
		logger.Instance().
			WithError(err).
			Warning("Unable to bind the queue")

		return
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		logger.Instance().
			WithError(err).
			Warning("Unable to start consume from the queue")

		return
	}

	// Now run the consumer
	go func() {
		var err error

		for m := range msgs {
			select {
			case <-die:
				// Before exiting coroutine, don't forget to store the message in queue
				c.storeMessage(m)
				return
			default:
			}

			c.storeMessage(m)
		}

		// Don't forget to close connection to RabbitMQ
		err = c.con.Close()
		if err != nil {
			logger.Instance().
				WithError(err).
				Warning("An error occurred while closing connection to RabbitMQ")
		}
	}()
}

// Stores the message in the queue
func (c *internalConsumer) storeMessage(m amqp.Delivery) {
	c.queue.add(internalMessage{
		Exchange:   m.Exchange,
		RoutingKey: m.RoutingKey,
		Body:       m.Body,
	})
}