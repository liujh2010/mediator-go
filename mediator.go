package mediator

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/dig"
)

var (
	_ IMediator = (*Mediator)(nil)

	ErrorNotEventHandler   = "not found handler for this event"
	ErrorNotCommandHandler = "not found handler for this command"
	ErrorContextTimeout    = "context time out"
)

func MediatorCtor() func(container *dig.Container, pool IRoutinePool) IMediator {
	return New
}

type Mediator struct {
	container         *dig.Container
	eventHandlerMap   map[reflect.Type]INotificationHandler
	commandHandlerMap map[reflect.Type]IRequestHandler
	pool              IRoutinePool
}

func New(container *dig.Container, pool IRoutinePool) IMediator {
	return &Mediator{
		container:         container,
		eventHandlerMap:   make(map[reflect.Type]INotificationHandler),
		commandHandlerMap: make(map[reflect.Type]IRequestHandler),
		pool:              pool,
	}
}

func (m *Mediator) Publish(ctx context.Context, event INotification) error {
	handler, ok := m.eventHandlerMap[event.Type()]
	if !ok {
		return errors.New(fmt.Sprintf("Publish: %s -> %v", ErrorNotEventHandler, event.Type()))
	}

	done := make(chan struct{})
	var err error

	m.pool.Publish(func() {
		// for {
		// 	select {
		// 	case handleErr := handler.Handle(event):
		// 		err = handleErr
		// 		close(done)
		// 	case <-ctx.Done():
		// 		err = errors.New(fmt.Sprintf("Publish: %s -> %v", ErrorContextTimeout, event.Type()))
		// 	}
		// }
	})
	return err
}
func (m *Mediator) Send(ctx context.Context, command IRequest) (interface{}, error) {
	return nil, nil
}
