package mediator_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/liujh2010/mediator"
	"github.com/panjf2000/ants/v2"
)

type TestRoutinePool struct {
}

func (p *TestRoutinePool) Publish(t ITask) error {
	go t()
	return nil
}

type TestEvent1 struct {
	msg string
}

type TestEvent1Handler struct{}

func (e *TestEvent1) Type() reflect.Type {
	return reflect.TypeOf(e)
}

func (h *TestEvent1Handler) Handle(ctx context.Context, event INotification) error {
	e := event.(*TestEvent1)
	e.msg += " 1 visited"
	return nil
}

type TestEvent2 struct {
	msg      string
	handler1 bool
	handler2 bool
	handler3 bool
}

type TestEvent2Handler struct{}

func (e *TestEvent2) Type() reflect.Type {
	return reflect.TypeOf(e)
}

func (h *TestEvent2Handler) Handle(ctx context.Context, event INotification) error {
	e := event.(*TestEvent2)
	e.msg += " 2 visited"
	e.handler1 = true
	return nil
}

type TestEvent2Handler2 struct{}

func (h *TestEvent2Handler2) Handle(ctx context.Context, event INotification) error {
	e := event.(*TestEvent2)
	e.msg += "[TestEvent2Handler2]"
	e.handler2 = true
	return nil
}

type TestEvent2Handler3 struct{}

func (h *TestEvent2Handler3) Handle(ctx context.Context, event INotification) error {
	e := event.(*TestEvent2)
	e.msg += "[TestEvent2Handler3]"
	e.handler3 = true
	return nil
}

type TestEvent2HandlerWithError1 struct{}

func (h *TestEvent2HandlerWithError1) Handle(ctx context.Context, event INotification) error {
	return errors.New("TestEvent2HandlerWithError1")
}

type TestEvent2HandlerWithError2 struct{}

func (h *TestEvent2HandlerWithError2) Handle(ctx context.Context, event INotification) error {
	return errors.New("TestEvent2HandlerWithError2")
}

func TestEvent(t *testing.T) {
	t.Run("event testing", func(t *testing.T) {
		builder := New(SetRoutinePool(new(TestRoutinePool)))
		builder.RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler))
		mediator := builder.Build()

		msg := "Testing"
		event := &TestEvent1{msg: msg}
		err := mediator.Publish(context.Background(), event)
		if err != nil {
			t.Errorf("got error: %+v", err)
		}
		msg += " 1 visited"
		if event.msg != msg {
			t.Error("result not match")
		}
	})

	t.Run("mutil evet testing", func(t *testing.T) {
		builder := New(SetRoutinePool(new(TestRoutinePool)))
		builder.RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler))
		builder.RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler))
		mediator := builder.Build()

		msg := "Testing"
		event1 := &TestEvent1{msg: msg}
		event2 := &TestEvent2{msg: msg}
		err := mediator.Publish(context.Background(), event1)
		if err != nil {
			t.Errorf("got error: %+v", err)
		}
		err = mediator.Publish(context.Background(), event2)
		if err != nil {
			t.Errorf("got error: %+v", err)
		}

		res1 := msg + " 1 visited"
		if event1.msg != res1 {
			t.Error("result not match")
		}
		res2 := msg + " 2 visited"
		if event2.msg != res2 {
			t.Error("result not match")
		}
	})

	t.Run("concurrency event testing", func(t *testing.T) {
		builder := New(SetRoutinePool(new(TestRoutinePool)))
		builder.RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler))
		builder.RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler))
		mediator := builder.Build()
		wg := sync.WaitGroup{}
		wg.Add(100)

		for i := 0; i < 100; i++ {
			go func() {
				for i := 0; i < 100; i++ {
					msg := "Testing"
					event1 := &TestEvent1{msg: msg}
					event2 := &TestEvent2{msg: msg}
					err := mediator.Publish(context.Background(), event1)
					if err != nil {
						t.Errorf("got error: %+v", err)
					}
					err = mediator.Publish(context.Background(), event2)
					if err != nil {
						t.Errorf("got error: %+v", err)
					}

					res1 := msg + " 1 visited"
					if event1.msg != res1 {
						t.Error("result not match")
					}
					res2 := msg + " 2 visited"
					if event2.msg != res2 {
						t.Error("result not match")
					}
				}
				wg.Done()
			}()
		}

		wg.Wait()
	})

	t.Run("mutil handler test", func(t *testing.T) {
		builder := New(SetRoutinePool(new(TestRoutinePool)))
		builder.RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler))
		builder.RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler))
		builder.RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2))
		builder.RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3))
		mediator := builder.Build()

		msg := "just testing"
		event := &TestEvent2{msg: msg}
		err := mediator.Publish(context.Background(), event)
		if err != nil {
			t.Errorf("got error: %+v", err)
		}
		if event.handler1 != true || event.handler2 != true || event.handler3 != true {
			t.Error("some handler are missing")
		}
	})

	t.Run("mutil handler error test", func(t *testing.T) {
		mediator := New(SetRoutinePool(new(TestRoutinePool))).
			RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2HandlerWithError1)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2HandlerWithError2)).
			Build()

		msg := "mutil errors testing"
		event := &TestEvent2{msg: msg}
		err := mediator.Publish(context.Background(), event)
		if err == nil {
			t.Errorf("expect a error but not got")
		}
		expectErrMsg := "TestEvent2HandlerWithError1; TestEvent2HandlerWithError2"
		if err.Error() != expectErrMsg {
			t.Errorf("wrong error msg, expect: %v, actual: %v", expectErrMsg, err)
		}
	})
}

type BlockEvent struct{}

func (e *BlockEvent) Type() reflect.Type {
	return reflect.TypeOf(new(BlockEvent))
}

type BlockEventHandler struct{}

func (e *BlockEventHandler) Handle(ctx context.Context, event INotification) error {
	time.Sleep(time.Second * 10000)
	return nil
}

func TestContext(t *testing.T) {
	builder := New(SetRoutinePool(new(TestRoutinePool)))
	mediator := builder.
		RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
		RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
		Build()

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err := mediator.Publish(ctx, new(BlockEvent))
	if err != context.DeadlineExceeded {
		t.Errorf("got wrong error， except: %v, actual: %v", context.DeadlineExceeded, err)
	}
}

type TestCommandCommon struct {
	msg      string
	duration time.Duration
	err      error
}

func (e *TestCommandCommon) Type() reflect.Type {
	return reflect.TypeOf(new(TestCommandCommon))
}

type TestCommandCommonHandler struct{}

func (e *TestCommandCommonHandler) Handle(ctx context.Context, command IRequest) (interface{}, error) {
	c := command.(*TestCommandCommon)
	time.Sleep(c.duration)
	if c.err != nil {
		return nil, c.err
	}
	return c.msg + " 1 visited", nil
}

type TestCommand1 struct {
	msg string
}

func (e *TestCommand1) Type() reflect.Type {
	return reflect.TypeOf(new(TestCommand1))
}

type TestCommand1Handler struct{}

func (e *TestCommand1Handler) Handle(ctx context.Context, command IRequest) (interface{}, error) {
	c := command.(*TestCommand1)
	return c.msg + " 2 visited", nil
}

func TestCommand(t *testing.T) {
	t.Run("command test", func(t *testing.T) {
		mediator := New(SetRoutinePool(new(TestRoutinePool))).
			RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
			RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
			RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
			Build()

		testMsg := "testing"
		res, err := mediator.Send(context.Background(), &TestCommandCommon{
			msg:      testMsg,
			duration: time.Microsecond,
			err:      nil,
		})

		if err != nil {
			t.Errorf("got a error when testing command: %v", err)
		}

		if res.(string) != (testMsg + " 1 visited") {
			t.Errorf("result doesn't match")
		}
	})

	t.Run("command error test", func(t *testing.T) {
		mediator := New(SetRoutinePool(new(TestRoutinePool))).
			RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
			RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
			RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
			Build()

		testMsg := "testing"
		testErr := errors.New("this is test error")
		res, err := mediator.Send(context.Background(), &TestCommandCommon{
			msg:      testMsg,
			duration: time.Microsecond,
			err:      testErr,
		})

		if res != nil {
			t.Errorf("result should be nil")
		}

		if err != testErr {
			t.Errorf("error doesn't match. expect: %v, actual: %v", testErr, err)
		}
	})

	t.Run("mulit command test", func(t *testing.T) {
		mediator := New(SetRoutinePool(new(TestRoutinePool))).
			RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
			RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
			RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
			RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
			Build()

		wg := &sync.WaitGroup{}
		wg.Add(3)
		testMsg := "testing"
		testErr := errors.New("this is test error")
		go func() {
			res, err := mediator.Send(context.Background(), &TestCommandCommon{
				msg:      testMsg,
				duration: time.Microsecond,
				err:      testErr,
			})

			if res != nil {
				t.Errorf("result should be nil")
			}

			if err != testErr {
				t.Errorf("error doesn't match. expect: %v, actual: %v", testErr, err)
			}
			wg.Done()
		}()

		go func() {
			res, err := mediator.Send(context.Background(), &TestCommandCommon{
				msg:      testMsg,
				duration: time.Microsecond,
				err:      nil,
			})

			if err != nil {
				t.Errorf("got a error when testing command: %v", err)
			}

			if res.(string) != (testMsg + " 1 visited") {
				t.Errorf("result should be nil")
			}
			wg.Done()
		}()

		go func() {
			res, err := mediator.Send(context.Background(), &TestCommand1{msg: testMsg})

			if err != nil {
				t.Errorf("got a error when testing command: %v", err)
			}

			if res.(string) != (testMsg + " 2 visited") {
				t.Errorf("result should be nil")
			}
			wg.Done()
		}()
		wg.Wait()
	})

	t.Run("command timeout test", func(t *testing.T) {
		mediator := New(SetRoutinePool(new(TestRoutinePool))).
			RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
			RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
			RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
			RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
			Build()

		testMsg := "testing"
		testErr := errors.New("this is test error")
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		res, err := mediator.Send(ctx, &TestCommandCommon{
			msg:      testMsg,
			duration: time.Second * 10000,
			err:      testErr,
		})

		if res != nil {
			t.Errorf("result should be nil")
		}

		if err != context.DeadlineExceeded {
			t.Errorf("error doesn't match. expect: %v, actual: %v", testErr, err)
		}
	})
}

type NotHandlerEvent struct{}

func (e *NotHandlerEvent) Type() reflect.Type {
	return reflect.TypeOf(new(NotHandlerEvent))
}

type NotHandlerCommand struct{}

func (e *NotHandlerCommand) Type() reflect.Type {
	return reflect.TypeOf(new(NotHandlerCommand))
}

func TestMediator(t *testing.T) {
	mediator := New(SetRoutinePool(new(TestRoutinePool))).
		RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
		RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
		RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
		RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
		Build()

	t.Run("not mapping event test", func(t *testing.T) {
		err := mediator.Publish(context.TODO(), new(NotHandlerEvent))
		if !strings.Contains(err.Error(), ErrorNotEventHandler) {
			t.Errorf("error not contains message, expect: %v, actual: %v", ErrorNotEventHandler, err.Error())
		}
	})

	t.Run("not mapping command test", func(t *testing.T) {
		_, err := mediator.Send(context.TODO(), new(NotHandlerCommand))
		if !strings.Contains(err.Error(), ErrorNotCommandHandler) {
			t.Errorf("error not contains message, expect: %v, actual: %v", ErrorNotCommandHandler, err.Error())
		}
	})

	t.Run("register event validation test", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Error("expect panic when using nil")
			}

			if !strings.Contains(err.(error).Error(), ErrorInvalidArgument) {
				t.Errorf("wrong error message, want: %v, got: %v", ErrorInvalidArgument, err)
			}
		}()

		mediator.(IMediatorBuilder).RegisterEventHandler(nil, nil)
	})

	t.Run("register command validation test", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Error("expect panic when using nil")
			}

			if !strings.Contains(err.(error).Error(), ErrorInvalidArgument) {
				t.Errorf("wrong error message, want: %v, got: %v", ErrorInvalidArgument, err)
			}
		}()

		mediator.(IMediatorBuilder).RegisterCommandHandler(nil, new(TestCommand1Handler))
	})

	t.Run("publish event validation test", func(t *testing.T) {
		err := mediator.Publish(context.TODO(), nil)

		if err == nil {
			t.Error("expect error when using nil")
		}

		if !strings.Contains(err.(error).Error(), ErrorInvalidArgument) {
			t.Errorf("wrong error message, want: %v, got: %v", ErrorInvalidArgument, err)
		}
	})

	t.Run("send command validation test", func(t *testing.T) {
		_, err := mediator.Send(nil, new(TestCommand1))

		if err == nil {
			t.Error("expect error when using nil")
		}

		if !strings.Contains(err.(error).Error(), ErrorInvalidArgument) {
			t.Errorf("wrong error message, want: %v, got: %v", ErrorInvalidArgument, err)
		}
	})
}

type PanicEvent struct {
	msg string
}

func (e *PanicEvent) Type() reflect.Type {
	return reflect.TypeOf(new(PanicEvent))
}

type PanicEventHandler struct{}

func (h *PanicEventHandler) Handle(ctx context.Context, event INotification) error {
	panic(event.(*PanicEvent).msg)
}

func TestDefaultRoutinePool(t *testing.T) {
	mediator := New(
		SetRoutinePool(
			NewRoutinePool(
				new(DefaultLogger),
				SetInitialPoolSize(50),
				SetMaxPoolSize(200),
				SetSubmitRetryCount(5),
				SetIsBlockingPool(true),
			),
		),
	).
		RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
		RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
		RegisterEventHandler(new(PanicEvent).Type(), new(PanicEventHandler)).
		RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
		RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
		Build()

	t.Run("event test", func(t *testing.T) {
		msg := "just testing"
		event := &TestEvent2{msg: msg}
		err := mediator.Publish(context.Background(), event)
		if err != nil {
			t.Errorf("got error: %+v", err)
		}
		if event.handler1 != true || event.handler2 != true || event.handler3 != true {
			t.Error("some handler are missing")
		}
	})

	t.Run("concurrency test", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(200)

		for i := 0; i < 200; i++ {
			go func() {
				for i := 0; i < 100; i++ {
					msg := "Testing"

					event1 := &TestEvent1{msg: msg}
					err := mediator.Publish(context.Background(), event1)
					if err != nil {
						t.Errorf("got error: %+v", err)
					}
					res, err := mediator.Send(context.Background(), &TestCommandCommon{
						msg: msg,
						err: nil,
					})
					if err != nil {
						t.Errorf("got a error when testing command: %v", err)
					}

					res1 := msg + " 1 visited"
					if event1.msg != res1 {
						t.Error("result not match")
					}
					if res.(string) != (msg + " 1 visited") {
						t.Errorf("result doesn't match")
					}
				}
				wg.Done()
			}()
		}

		wg.Wait()
	})

	t.Run("adjust capacity test", func(t *testing.T) {
		mediator := New(
			SetRoutinePool(
				NewRoutinePool(
					new(DefaultLogger),
					SetInitialPoolSize(10),
					SetMaxPoolSize(20),
					SetSubmitRetryCount(2),
				),
			),
		).
			RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
			RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
			RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
			RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
			Build()

		for i := 0; i < 20; i++ {
			go func() {
				mediator.Send(context.TODO(), &TestCommandCommon{
					msg:      "adjust test",
					duration: time.Millisecond * 500,
				})
			}()
		}

		time.Sleep(time.Millisecond * 100)
		_, err := mediator.Send(context.TODO(), &TestCommandCommon{
			msg: "adjust test",
		})
		if err != ants.ErrPoolOverload {
			t.Errorf("want got error %v, but got %v", ants.ErrPoolOverload, err)
		}
	})

	t.Run("pool gradient descent test", func(t *testing.T) {
		mediator := New(
			SetRoutinePool(
				NewRoutinePool(
					new(DefaultLogger),
					SetInitialPoolSize(20),
					SetMaxPoolSize(20),
					SetSubmitRetryCount(6),
				),
			),
		).
			RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
			RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
			RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
			RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
			RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
			Build()

		wg := &sync.WaitGroup{}
		wg.Add(20)
		for i := 0; i < 20; i++ {
			go func() {
				wg.Done()
				mediator.Send(context.TODO(), &TestCommandCommon{
					msg:      "adjust test",
					duration: time.Millisecond * 50,
				})
			}()
		}

		wg.Wait()
		time.Sleep(time.Millisecond * 10)
		_, err := mediator.Send(context.TODO(), &TestCommandCommon{
			msg: "adjust test",
		})
		if err != nil {
			t.Errorf("got error %+v", err)
		}
	})

	t.Run("panic test", func(t *testing.T) {
		panicEvent := &PanicEvent{msg: "just panic"}
		err := mediator.Publish(context.TODO(), panicEvent)
		expectErrMsg := "got panic when running *mediator_test.PanicEvent event, cause: " + panicEvent.msg
		if err.Error() != expectErrMsg {
			t.Errorf("error not match, expect: %v, actual : %v", expectErrMsg, err.Error())
		}
	})
}
