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

	// INotificationHandler ...
	INotificationHandler interface {
		Handle(ctx context.Context, event INotification) error
	}

	// IRequest ...
	IRequest interface {
		INotification
	}

	// IRequestHandler ...
	IRequestHandler interface {
		Handle(ctx context.Context, command IRequest) (interface{}, error)
	}

	// IMediator ...
	IMediator interface {
		Publish(ctx context.Context, event INotification) IResult
		Send(ctx context.Context, command IRequest) IResult
	}

	// IMediatorBuilder ...
	IMediatorBuilder interface {
		RegisterEventHandler(matchingType reflect.Type, eventHandler INotificationHandler) IMediatorBuilder
		RegisterCommandHandler(matchingType reflect.Type, commandHandler IRequestHandler) IMediatorBuilder
		Build() IMediator
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
)
