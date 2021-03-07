package mediator

import (
	"context"
	"fmt"
	"log"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
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
)

type (
	// Mediator ...
	Mediator struct {
		mediatorConfig
		mut                    *sync.Mutex
		eventHandlerMap        map[reflect.Type][]INotificationHandler
		commandHandlerMap      map[reflect.Type]IRequestHandler
		behaviorPipelineRunner *behaviorPipelineRunner
	}

	mediatorConfig struct {
		logger ILogger
		pool   IRoutinePool
	}

	// Option ...
	Option interface {
		applyOption(config *mediatorConfig)
	}

	// OptionFunc ...
	OptionFunc func(config *mediatorConfig)
)

func (f OptionFunc) applyOption(config *mediatorConfig) { f(config) }

// SetLogger ...
func SetLogger(logger ILogger) Option {
	return OptionFunc(func(config *mediatorConfig) {
		config.logger = logger
	})
}

// SetRoutinePool ...
func SetRoutinePool(pool IRoutinePool) Option {
	return OptionFunc(func(config *mediatorConfig) {
		config.pool = pool
	})
}

// Options ...
func Options(options ...Option) Option {
	return OptionFunc(func(config *mediatorConfig) {
		for _, option := range options {
			option.applyOption(config)
		}
	})
}

// New ...
func New(options ...Option) IMediatorBuilder {
	config := &mediatorConfig{
		logger: new(DefaultLogger),
	}

	Options(options...).applyOption(config)
	if config.pool == nil {
		config.pool = NewRoutinePool(config.logger)
	}

	mediator := &Mediator{
		mut:                    &sync.Mutex{},
		eventHandlerMap:        make(map[reflect.Type][]INotificationHandler),
		commandHandlerMap:      make(map[reflect.Type]IRequestHandler),
		behaviorPipelineRunner: &behaviorPipelineRunner{},
		mediatorConfig:         *config,
	}

	mediator.behaviorPipelineRunner.register(newBehaviorHandlerWrapper(
		func(ctx context.Context, command IRequest, next func(ctx context.Context) IResultContext) IResultContext {
			return mediator.send(ctx, command)
		},
	))

	return mediator
}

// Publish ...
func (m *Mediator) Publish(ctx context.Context, event INotification) IResult {
	result := &Result{}
	if ctx == nil || event == nil {
		return result.SetErr(errors.New((ErrorInvalidArgument + " ctx or event")))
	}

	handlers, ok := m.eventHandlerMap[event.Type()]
	if !ok {
		return result.SetErr(
			errors.Errorf("Publish: %s -> %v", ErrorNotEventHandler, event.Type().String()),
		)
	}

	var (
		doneSilce []chan struct{}
		errNoti   = newErrorNotifacation()
	)

	for _, handler := range handlers {

		done := make(chan struct{})
		doneSilce = append(doneSilce, done)

		if poolErr := func(h INotificationHandler) error {

			return m.pool.Publish(func() {
				defer func() {
					if internalErr := recover(); internalErr != nil {
						msg := fmt.Sprintf("got panic when running %v event, cause: %v", event.Type().String(), internalErr)
						m.logger.Errorf(msg)
						errNoti.add(errors.New(msg))
						close(done)
					}
				}()

				err := h.Handle(ctx, event)
				errNoti.add(err)
				close(done)
			})

		}(handler); poolErr != nil {
			return result.SetErr(poolErr)
		}

	}

	select {
	case <-waitAllDone(doneSilce):
		if errNoti.HasError() {
			return result.SetErr(errNoti)
		}
		return result
	case <-ctx.Done():
		return result.SetErr(ctx.Err())
	}
}

// Send ...
func (m *Mediator) Send(ctx context.Context, command IRequest) IResult {
	return m.behaviorPipelineRunner.run(ctx, command)
}

func (m *Mediator) send(ctx context.Context, command IRequest) IResultContext {
	result := &Result{}
	if ctx == nil || command == nil {
		return result.SetErr(errors.New(ErrorInvalidArgument + " ctx or command"))
	}

	handler, ok := m.commandHandlerMap[command.Type()]
	if !ok {
		return result.SetErr(errors.Errorf("Send: %s -> %v", ErrorNotCommandHandler, command.Type().String()))
	}

	done := make(chan struct{})
	var (
		data interface{}
		err  error
	)

	if poolErr := m.pool.Publish(func() {
		defer func() {
			if internalErr := recover(); internalErr != nil {
				msg := fmt.Sprintf("got panic when running %v command, cause: %v", command.Type().String(), internalErr)
				m.logger.Errorf(msg)
				err = errors.New(msg)
				close(done)
			}
		}()

		data, err = handler.Handle(ctx, command)
		close(done)
	}); poolErr != nil {
		return result.SetErr(poolErr)
	}

	select {
	case <-done:
		return result.
			SetVal(data).
			SetErr(err)
	case <-ctx.Done():
		return result.SetErr(ctx.Err())
	}
}

// RegisterBehaviorHandler ...
func (m *Mediator) RegisterBehaviorHandler(behaviorHandler IBehaviorHandler) IMediatorBuilder {
	if behaviorHandler == nil {
		panic(errors.New(ErrorInvalidArgument + " behaviorHandler"))
	}

	m.mutex(func() {
		m.behaviorPipelineRunner.register(behaviorHandler)
	})
	return m
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
	if len(e.errors) < 1 {
		return ""
	}

	strBuilder := &strings.Builder{}
	for _, err := range e.errors {
		strBuilder.WriteString(fmt.Sprintf("%+v\n", err))
	}
	return strBuilder.String()
}

const (
	// DefaultPoolCap ...
	DefaultPoolCap = 1000

	// DefaultMaxPoolCap ...
	DefaultMaxPoolCap = 10000

	// DefualtPoolSubmitRetryCount ...
	DefualtPoolSubmitRetryCount = 5

	// DefaultIsBlockingPool ...
	DefaultIsBlockingPool = false
)

var _ IRoutinePool = (*DefaultRoutinePool)(nil)

type (
	// DefaultRoutinePool ...
	DefaultRoutinePool struct {
		poolConfig
		logger ILogger
		pool   *ants.Pool
	}

	poolConfig struct {
		initialPoolSize  int
		maxPoolSize      int
		submitRetryCount int
		isBlockingPool   bool
		logger           ILogger
	}

	// DefaultRoutinePoolOption ...
	DefaultRoutinePoolOption interface {
		applyOption(config *poolConfig)
	}

	// DefaultRoutinePoolOptionFunc ...
	DefaultRoutinePoolOptionFunc func(config *poolConfig)
)

func (f DefaultRoutinePoolOptionFunc) applyOption(config *poolConfig) { f(config) }

// SetInitialPoolSize ...
func SetInitialPoolSize(size int) DefaultRoutinePoolOption {
	return DefaultRoutinePoolOptionFunc(func(config *poolConfig) {
		config.initialPoolSize = size
	})
}

// SetMaxPoolSize ...
func SetMaxPoolSize(size int) DefaultRoutinePoolOption {
	return DefaultRoutinePoolOptionFunc(func(config *poolConfig) {
		config.maxPoolSize = size
	})
}

// SetSubmitRetryCount ...
func SetSubmitRetryCount(count int) DefaultRoutinePoolOption {
	return DefaultRoutinePoolOptionFunc(func(config *poolConfig) {
		config.submitRetryCount = count
	})
}

// SetIsBlockingPool ...
func SetIsBlockingPool(blocking bool) DefaultRoutinePoolOption {
	return DefaultRoutinePoolOptionFunc(func(config *poolConfig) {
		config.isBlockingPool = blocking
	})
}

// PoolOptions ...
func PoolOptions(options ...DefaultRoutinePoolOption) DefaultRoutinePoolOption {
	return DefaultRoutinePoolOptionFunc(func(config *poolConfig) {
		for _, v := range options {
			v.applyOption(config)
		}
	})
}

// NewRoutinePool ...
func NewRoutinePool(logger ILogger, options ...DefaultRoutinePoolOption) *DefaultRoutinePool {
	config := &poolConfig{
		initialPoolSize:  DefaultPoolCap,
		maxPoolSize:      DefaultMaxPoolCap,
		submitRetryCount: DefualtPoolSubmitRetryCount,
		isBlockingPool:   DefaultIsBlockingPool,
	}
	PoolOptions(options...).applyOption(config)

	pool, err := ants.NewPool(
		config.initialPoolSize,
		ants.WithPanicHandler(func(i interface{}) {
			logger.Errorf("mediator: got a panic when running handler: %v", i)
		}),
		ants.WithPreAlloc(false),
		ants.WithNonblocking(!config.isBlockingPool),
	)
	if err != nil {
		panic("can not initialize the pool: " + err.Error())
	}

	return &DefaultRoutinePool{
		pool:       pool,
		logger:     logger,
		poolConfig: *config,
	}
}

// Publish ...
func (p *DefaultRoutinePool) Publish(t ITask) error {
	var err error
	for i := 1; i <= p.submitRetryCount; i++ {
		err = p.pool.Submit(t)
		if err == nil {
			return nil
		} else if err == ants.ErrPoolOverload && p.pool.Cap() < p.maxPoolSize {

			newCap := int(math.Min(float64(p.pool.Cap()*2), float64(DefaultMaxPoolCap)))
			p.pool.Tune(newCap)
			p.logger.Printf("routine pool overload, expansion to pool cap: %d", newCap)

		} else if err == ants.ErrPoolOverload {

			p.logger.Printf("routine pool overload, and the capacity has reached the set maximum. Retry after sleep %dms, retry for the %dth time", time.Duration(i*i), i-1)
			// gradient descent
			time.Sleep(time.Millisecond * time.Duration(math.Pow(2.0, float64(i))))

		} else {
			p.logger.Errorf("routine pool error: %v", err)
			return err
		}
	}

	if err != nil {
		p.logger.Errorf("routine pool error: %v", err)
	}
	return err
}

var _ ILogger = (*DefaultLogger)(nil)

// DefaultLogger ...
type DefaultLogger struct{}

// Printf ...
func (l *DefaultLogger) Printf(format string, messages ...interface{}) {
	log.Printf(format, messages...)
}

// Errorf ...
func (l *DefaultLogger) Errorf(format string, messages ...interface{}) {
	log.Printf(format, messages...)
}

var (
	_ IResult        = (*Result)(nil)
	_ IResultContext = (*Result)(nil)
)

// Result ...
type Result struct {
	err   error
	value interface{}
}

// Err ...
func (r Result) Err() error {
	return r.err
}

// Value ...
func (r Result) Value() interface{} {
	return r.value
}

// ValueT ...
func (r Result) ValueT(ptr interface{}) {
	reflect.ValueOf(ptr).Elem().Set(reflect.ValueOf(r.value))
}

// HasError ...
func (r Result) HasError() bool {
	return r.err != nil
}

// HasValue ...
func (r Result) HasValue() bool {
	return r.value != nil
}

// SetErr ...
func (r *Result) SetErr(err error) IResultContext {
	r.err = err
	return r
}

// SetVal ...
func (r *Result) SetVal(val interface{}) IResultContext {
	r.value = val
	return r
}

var (
	// _ IBehaviorHandler = (*DefaultBehaviorHandler)(nil)
	_ IBehaviorHandler = (*behaviorHandlerWrapper)(nil)
)

type (
	behaviorPipelineRunner struct {
		behaviors []IBehaviorHandler
	}
	behaviorHandlerWrapper BehaviorHandlerFunc
)

func (r *behaviorPipelineRunner) register(handler IBehaviorHandler) *behaviorPipelineRunner {
	newBehaviorsChain := make([]IBehaviorHandler, len(r.behaviors)+1)
	newBehaviorsChain[0] = handler

	for i, h := range r.behaviors {
		newBehaviorsChain[i+1] = h
	}
	r.behaviors = newBehaviorsChain

	return r
}

func (r *behaviorPipelineRunner) run(ctx context.Context, command IRequest) IResultContext {
	next := r.buildNext(0, command)
	return next(ctx)
}

func (r *behaviorPipelineRunner) buildNext(index int, command IRequest) func(ctx context.Context) IResultContext {
	return func(ctx context.Context) IResultContext {
		if index < len(r.behaviors) {
			return r.behaviors[index].Handle(ctx, command, r.buildNext(index+1, command))
		}
		return nil
	}
}

// NewDefaultBehaviorHandlerWrapper ...
func newBehaviorHandlerWrapper(fn BehaviorHandlerFunc) IBehaviorHandler {
	return behaviorHandlerWrapper(fn)
}

// Handle ...
func (fn behaviorHandlerWrapper) Handle(ctx context.Context, command IRequest, next func(ctx context.Context) IResultContext) IResultContext {
	return fn(ctx, command, next)
}
