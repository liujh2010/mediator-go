package mediator

import (
	"context"
	"reflect"
)

type (
	// INotification ...
	INotification interface {
		Type() reflect.Type
	}

	// IRequest ...
	IRequest interface {
		INotification
	}

	// INotificationHandler ...
	INotificationHandler interface {
		Handle(ctx context.Context, event INotification) error
	}

	// IRequestHandler ...
	IRequestHandler interface {
		Handle(ctx context.Context, command IRequest) (interface{}, error)
	}

	// BehaviorHandlerFunc ...
	BehaviorHandlerFunc func(ctx context.Context, command IRequest, next func(ctx context.Context) IResultContext) IResultContext

	// IBehaviorHandler ...
	IBehaviorHandler interface {
		Handle(ctx context.Context, command IRequest, next func(ctx context.Context) IResultContext) IResultContext
	}

	// IMediator ...
	IMediator interface {
		Publish(ctx context.Context, event INotification) IResult
		Send(ctx context.Context, command IRequest) IResult
	}

	// IMediatorBuilder ...
	IMediatorBuilder interface {
		Build() IMediator
		RegisterBehaviorHandler(handler IBehaviorHandler) IMediatorBuilder
		RegisterEventHandler(matchingType reflect.Type, eventHandler INotificationHandler) IMediatorBuilder
		RegisterCommandHandler(matchingType reflect.Type, commandHandler IRequestHandler) IMediatorBuilder
	}

	// ITask ...
	ITask func()

	// IRoutinePool ...
	IRoutinePool interface {
		Publish(t ITask) error
	}

	// ILogger ...
	ILogger interface {
		Printf(format string, messages ...interface{})
		Errorf(format string, messages ...interface{})
	}

	// IResult ...
	IResult interface {
		Err() error
		Value() interface{}
		ValueT(ptr interface{})
		HasError() bool
		HasValue() bool
	}

	// IResultContext ...
	IResultContext interface {
		IResult
		SetErr(err error) IResultContext
		SetVal(val interface{}) IResultContext
	}
)
