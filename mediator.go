package mediator

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

var (
	_ IMediator = (*Mediator)(nil)

	ErrorNotEventHandler   = "not found handler for this event"
	ErrorNotCommandHandler = "not found handler for this command"
	ErrorContextTimeout    = "context time out"
)

// MediatorCtor...
func MediatorCtor() func(pool IRoutinePool) IMediator {
	return New
}

// Mediator...
type Mediator struct {
	eventHandlerMap   map[reflect.Type]INotificationHandler
	commandHandlerMap map[reflect.Type]IRequestHandler
	pool              IRoutinePool
}

// New...
func New(pool IRoutinePool) IMediator {
	return &Mediator{
		eventHandlerMap:   make(map[reflect.Type]INotificationHandler),
		commandHandlerMap: make(map[reflect.Type]IRequestHandler),
		pool:              pool,
	}
}

// Publish...
func (m *Mediator) Publish(ctx context.Context, event INotification) error {
	handler, ok := m.eventHandlerMap[event.Type()]
	if !ok {
		return errors.New(fmt.Sprintf("Publish: %s -> %v", ErrorNotEventHandler, event.Type()))
	}

	done := make(chan struct{})
	var err error

	m.pool.Publish(func() {
		err = handler.Handle(event)
		close(done)
	})

	<-done
	return err
}

// Send...
func (m *Mediator) Send(ctx context.Context, command IRequest) (interface{}, error) {
	handler, ok := m.commandHandlerMap[command.Type()]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Publish: %s -> %v", ErrorNotEventHandler, event.Type()))
	}

	done := make(chan struct{})
	var (
		data interface{}
		err  error
	)

	m.pool.Publish(func() {
		data, err = handler.Handle(command)
		close(done)
	})

	<-done
	return data, err
}

// RegisterEventHandler...
func (m *Mediator) RegisterEventHandler(matchingType reflect.Type, eventHandler INotificationHandler) *Mediator {
	m.eventHandlerMap[matchingType] = eventHandler
	return m
}

// RegisterCommandHandler...
func (m *Mediator) RegisterCommandHandler(matchingType reflect.Type, commandHandler IRequestHandler) *Mediator {
	m.commandHandlerMap[matchingType] = commandHandler
	return m
}
