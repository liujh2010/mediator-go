package mediator_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/liujh2010/mediator"
)

type TestRoutinePool struct {
}

func (p *TestRoutinePool) Publish(t mediator.ITask) error {
	go t()
	return nil
}

type TestEvent struct {
	msg string
}

type TestEventHandler struct {
}

func (e *TestEvent) Type() reflect.Type {
	return reflect.TypeOf(e)
}

func (h *TestEventHandler) Handle(event mediator.INotification) error {
	e := event.(*TestEvent)
	e.msg += " visited"
	return nil
}

func TestMediator(t *testing.T) {
	builder := mediator.New(new(TestRoutinePool))
	builder.RegisterEventHandler(new(TestEvent).Type(), new(TestEventHandler))

	mediator := builder.Build()

	t.Run("event testing", func(t *testing.T) {
		msg := "Testing"
		event := &TestEvent{msg: msg}
		err := mediator.Publish(context.Background(), event)
		if err != nil {
			t.Errorf("got error: %+v", err)
		}
		msg += " visited"
		if event.msg != msg {
			t.Error("result not match")
		}
	})
}
