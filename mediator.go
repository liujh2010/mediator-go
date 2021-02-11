package mediator

import (
	"context"
	"log"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
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

	// ErrorInvalidArgument ...
	ErrorInvalidArgument = "invalid argument"

	// DefualtPoolCap ...
	DefualtPoolCap = 1000

	// MaxPoolCap ...
	MaxPoolCap = 10000
)

// Mediator ...
type Mediator struct {
	mut               *sync.Mutex
	eventHandlerMap   map[reflect.Type][]INotificationHandler
	commandHandlerMap map[reflect.Type]IRequestHandler
	pool              IRoutinePool
}

// New ...
func New() IMediatorBuilder {
	defaultPool := NewRoutinePool(new(DefaultLogger))
	return NewWithPool(defaultPool)
}

// NewWithPool ...
func NewWithPool(pool IRoutinePool) IMediatorBuilder {
	if pool == nil {
		panic("invalid argument pool")
	}
	return &Mediator{
		mut:               &sync.Mutex{},
		eventHandlerMap:   make(map[reflect.Type][]INotificationHandler),
		commandHandlerMap: make(map[reflect.Type]IRequestHandler),
		pool:              pool,
	}
}

// Publish ...
func (m *Mediator) Publish(ctx context.Context, event INotification) error {
	if ctx == nil || event == nil {
		return errors.New((ErrorInvalidArgument + " ctx or event"))
	}

	handlers, ok := m.eventHandlerMap[event.Type()]
	if !ok {
		return errors.Errorf("Publish: %s -> %v", ErrorNotEventHandler, event.Type().String())
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
		if errNoti.HasError() {
			return errNoti
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Send ...
func (m *Mediator) Send(ctx context.Context, command IRequest) (interface{}, error) {
	if ctx == nil || command == nil {
		return nil, errors.New(ErrorInvalidArgument + " ctx or command")
	}

	handler, ok := m.commandHandlerMap[command.Type()]
	if !ok {
		return nil, errors.Errorf("Send: %s -> %v", ErrorNotCommandHandler, command.Type().String())
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
	if matchingType == nil || eventHandler == nil {
		panic(errors.New(ErrorInvalidArgument + " matchingType or eventHandler"))
	}

	m.mutex(func() {
		m.eventHandlerMap[matchingType] = append(m.eventHandlerMap[matchingType], eventHandler)
	})
	return m
}

// RegisterCommandHandler ...
func (m *Mediator) RegisterCommandHandler(matchingType reflect.Type, commandHandler IRequestHandler) IMediatorBuilder {
	if matchingType == nil || commandHandler == nil {
		panic(errors.New(ErrorInvalidArgument + " matchingType or commandHandler"))
	}

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

func (e *ErrorNotification) Error() string {
	return multierr.Combine(e.errors...).Error()
}

// DefualtRoutinePool ...
type DefualtRoutinePool struct {
	pool *ants.Pool
}

// NewRoutinePool ...
func NewRoutinePool(logger ILogger) *DefualtRoutinePool {
	pool, err := ants.NewPool(1000,
		ants.WithPanicHandler(func(i interface{}) {
			logger.Errorf("mediator: got a panic when running handler: %v", i)
		}),
		ants.WithPreAlloc(false),
	)
	if err != nil {
		panic("can not initialize the pool: " + err.Error())
	}
	return &DefualtRoutinePool{
		pool: pool,
	}
}

// Publish ...
func (p *DefualtRoutinePool) Publish(t ITask) error {
	var err error
	for i := 1; i <= 5; i++ {
		err = p.pool.Submit(t)
		if err == nil {
			return nil
		} else if err == ants.ErrPoolOverload && p.pool.Cap() < MaxPoolCap {
			newCap := int(math.Min(float64(p.pool.Cap()*2), float64(MaxPoolCap)))
			p.pool.Tune(newCap)
		} else if err == ants.ErrPoolOverload {
			// gradient descent
			time.Sleep(time.Millisecond * time.Duration(i*i))
		} else {
			return err
		}
	}
	return err
}

// DefaultLogger ...
type DefaultLogger struct{}

// Printf ...
func (l *DefaultLogger) Printf(format string, messages ...interface{}) {
	log.Printf(format, messages...)
}

// Errorf ...
func (l *DefaultLogger) Errorf(format string, messages ...interface{}) {
	log.Fatalf(format, messages...)
}
