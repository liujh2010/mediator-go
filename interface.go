package mediator

import (
	"context"
	"reflect"
)

// INotification ...
type INotification interface {
	Type() reflect.Type
}

// INotificationHandler ...
type INotificationHandler interface {
	Handle(event INotification) error
}

// IRequest ...
type IRequest interface {
	INotification
}

// IRequestHandler ...
type IRequestHandler interface {
	Handle(command IRequest) (interface{}, error)
}

// IMediator ...
type IMediator interface {
	Publish(ctx context.Context, event INotification) error
	Send(ctx context.Context, command IRequest) (interface{}, error)
}

// IMediatorBuilder ...
type IMediatorBuilder interface {
	RegisterEventHandler(matchingType reflect.Type, eventHandler INotificationHandler) IMediatorBuilder
	RegisterCommandHandler(matchingType reflect.Type, commandHandler IRequestHandler) IMediatorBuilder
	Build() IMediator
}

// ITask ...
type ITask func()

// IRoutinePool ...
type IRoutinePool interface {
	Publish(t ITask) error
}

// ILogger ...
type ILogger interface {
	Printf(format string, messages ...interface{})
	Errorf(format string, messages ...interface{})
}
