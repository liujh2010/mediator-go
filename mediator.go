package mediator

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

var (
	_ IMediator        = (*Mediator)(nil)
	_ IMediatorBuilder = (*Mediator)(nil)
)

const (
	// ErrorNotEventHandler ...
	ErrorNotEventHandler = "not found handler for this event"

	// ErrorNotCommandHandler ...
	ErrorNotCommandHandler = "not found handler for this command"

	// ErrorContextTimeout ...
	ErrorContextTimeout = "context time out"
)

// Mediator ...
type Mediator struct {
	mut               *sync.Mutex
	eventHandlerMap   map[reflect.Type][]INotificationHandler
	commandHandlerMap map[reflect.Type]IRequestHandler
	pool              IRoutinePool
}

// New ...
func New(pool IRoutinePool) IMediatorBuilder {
	return &Mediator{
		mut:               &sync.Mutex{},
		eventHandlerMap:   make(map[reflect.Type][]INotificationHandler),
		commandHandlerMap: make(map[reflect.Type]IRequestHandler),
		pool:              pool,
	}
}

// Publish ...
func (m *Mediator) Publish(ctx context.Context, event INotification) error {
	handlers, ok := m.eventHandlerMap[event.Type()]
	if !ok {
		return errors.New(fmt.Sprintf("Publish: %s -> %v", ErrorNotEventHandler, event.Type().String()))
	}

	var (
		doneSilce []chan struct{}
		errNoti   = newErrorNotifacation()
	)

	for _, handler := range handlers {

		done := make(chan struct{})
		doneSilce = append(doneSilce, done)

		func(h INotificationHandler) {

			m.pool.Publish(func() {
				err := h.Handle(event)
				errNoti.add(err)
				close(done)
			})

		}(handler)
	}

	select {
	case <-waitAllDone(doneSilce):
		return errNoti.ToSingleError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Send ...
func (m *Mediator) Send(ctx context.Context, command IRequest) (interface{}, error) {
	handler, ok := m.commandHandlerMap[command.Type()]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Publish: %s -> %v", ErrorNotCommandHandler, command.Type().String()))
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

	select {
	case <-done:
		return data, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// RegisterEventHandler ...
func (m *Mediator) RegisterEventHandler(matchingType reflect.Type, eventHandler INotificationHandler) IMediatorBuilder {
	m.mutex(func() {
		m.eventHandlerMap[matchingType] = append(m.eventHandlerMap[matchingType], eventHandler)
	})
	return m
}

// RegisterCommandHandler ...
func (m *Mediator) RegisterCommandHandler(matchingType reflect.Type, commandHandler IRequestHandler) IMediatorBuilder {
	m.mutex(func() {
		m.commandHandlerMap[matchingType] = commandHandler
	})
	return m
}

// Build ...
func (m *Mediator) Build() IMediator {
	return m
}

func (m *Mediator) mutex(fn func()) {
	m.mut.Lock()
	fn()
	m.mut.Unlock()
}

func waitAllDone(doneSlice []chan struct{}) <-chan struct{} {
	allDone := make(chan struct{})
	go func() {
		for _, done := range doneSlice {
			<-done
		}
		close(allDone)
	}()
	return allDone
}

// ErrorNotification ...
type ErrorNotification struct {
	mut    *sync.Mutex
	errors []error
}

func newErrorNotifacation() *ErrorNotification {
	return &ErrorNotification{
		mut: &sync.Mutex{},
	}
}

func (e *ErrorNotification) add(err error) {
	if err == nil {
		return
	}

	e.mut.Lock()
	e.errors = append(e.errors, err)
	e.mut.Unlock()
}

// HasError ...
func (e *ErrorNotification) HasError() bool {
	return len(e.errors) > 0
}

// Errors ...
func (e *ErrorNotification) Errors() []error {
	return e.errors
}

// ToSingleError ...
func (e *ErrorNotification) ToSingleError() error {
	return multierr.Combine(e.errors...)
}

func (e *ErrorNotification) Error() string {
	return multierr.Combine(e.errors...).Error()
}
