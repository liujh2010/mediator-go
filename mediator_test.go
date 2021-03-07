package mediator_test

import (
	"context"
	"errors"
	"log"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	pkgerr "github.com/pkg/errors"

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
		result := mediator.Publish(context.Background(), event)
		if result.Err() != nil {
			t.Errorf("got error: %+v", result.Err())
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
		result := mediator.Publish(context.Background(), event1)
		if result.Err() != nil {
			t.Errorf("got error: %+v", result.Err())
		}
		result = mediator.Publish(context.Background(), event2)
		if result.Err() != nil {
			t.Errorf("got error: %+v", result.Err())
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
					result := mediator.Publish(context.Background(), event1)
					if result.Err() != nil {
						t.Errorf("got error: %+v", result.Err())
					}
					result = mediator.Publish(context.Background(), event2)
					if result.Err() != nil {
						t.Errorf("got error: %+v", result.Err())
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
		result := mediator.Publish(context.Background(), event)
		if result.Err() != nil {
			t.Errorf("got error: %+v", result.Err())
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
		result := mediator.Publish(context.Background(), event)
		if result.Err() == nil {
			t.Errorf("expect a error but not got")
		}
		expectErrMsg := "TestEvent2HandlerWithError1\nTestEvent2HandlerWithError2\n"
		if result.Err().Error() != expectErrMsg {
			errStr := result.Err().Error()
			t.Errorf("wrong error msg, expect: %v, actual: %v", expectErrMsg, errStr)
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
	result := mediator.Publish(ctx, new(BlockEvent))
	if result.Err() != context.DeadlineExceeded {
		t.Errorf("got wrong error， except: %v, actual: %v", context.DeadlineExceeded, result.Err())
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
		result := mediator.Send(context.Background(), &TestCommandCommon{
			msg:      testMsg,
			duration: time.Microsecond,
			err:      nil,
		})

		if result.Err() != nil {
			t.Errorf("got a error when testing command: %v", result.Err())
		}

		if result.Value().(string) != (testMsg + " 1 visited") {
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
		result := mediator.Send(context.Background(), &TestCommandCommon{
			msg:      testMsg,
			duration: time.Microsecond,
			err:      testErr,
		})

		if result.Value() != nil {
			t.Errorf("value should be nil")
		}

		if result.Err() != testErr {
			t.Errorf("error doesn't match. expect: %v, actual: %v", testErr, result.Err())
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
			result := mediator.Send(context.Background(), &TestCommandCommon{
				msg:      testMsg,
				duration: time.Microsecond,
				err:      testErr,
			})

			if result.Value() != nil {
				t.Errorf("value should be nil")
			}

			if result.Err() != testErr {
				t.Errorf("error doesn't match. expect: %v, actual: %v", testErr, result.Err())
			}
			wg.Done()
		}()

		go func() {
			result := mediator.Send(context.Background(), &TestCommandCommon{
				msg:      testMsg,
				duration: time.Microsecond,
				err:      nil,
			})

			if result.Err() != nil {
				t.Errorf("got a error when testing command: %v", result.Err())
			}

			if result.Value().(string) != (testMsg + " 1 visited") {
				t.Errorf("result should be nil")
			}
			wg.Done()
		}()

		go func() {
			result := mediator.Send(context.Background(), &TestCommand1{msg: testMsg})

			if result.Err() != nil {
				t.Errorf("got a error when testing command: %v", result.Err())
			}

			if result.Value().(string) != (testMsg + " 2 visited") {
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
		result := mediator.Send(ctx, &TestCommandCommon{
			msg:      testMsg,
			duration: time.Second * 10000,
			err:      testErr,
		})

		if result.Value() != nil {
			t.Errorf("result should be nil")
		}

		if result.Err() != context.DeadlineExceeded {
			t.Errorf("error doesn't match. expect: %v, actual: %v", testErr, result.Err())
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
		result := mediator.Publish(context.TODO(), new(NotHandlerEvent))
		if !strings.Contains(result.Err().Error(), ErrorNotEventHandler) {
			t.Errorf("error not contains message, expect: %v, actual: %v", ErrorNotEventHandler, result.Err().Error())
		}
	})

	t.Run("not mapping command test", func(t *testing.T) {
		result := mediator.Send(context.TODO(), new(NotHandlerCommand))
		if !strings.Contains(result.Err().Error(), ErrorNotCommandHandler) {
			t.Errorf("error not contains message, expect: %v, actual: %v", ErrorNotCommandHandler, result.Err().Error())
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
		result := mediator.Publish(context.TODO(), nil)

		if result.Err() == nil {
			t.Error("expect error when using nil")
		}

		if !strings.Contains(result.Err().Error(), ErrorInvalidArgument) {
			t.Errorf("wrong error message, want: %v, got: %v", ErrorInvalidArgument, result.Err().Error())
		}
	})

	t.Run("send command validation test", func(t *testing.T) {
		result := mediator.Send(nil, new(TestCommand1))

		if result.Err() == nil {
			t.Error("expect error when using nil")
		}

		if !strings.Contains(result.Err().Error(), ErrorInvalidArgument) {
			t.Errorf("wrong error message, want: %v, got: %v", ErrorInvalidArgument, result.Err())
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
		result := mediator.Publish(context.Background(), event)
		if result.Err() != nil {
			t.Errorf("got error: %+v", result.Err())
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
					result1 := mediator.Publish(context.Background(), event1)
					if result1.Err() != nil {
						t.Errorf("got error: %+v", result1.Err())
					}
					result2 := mediator.Send(context.Background(), &TestCommandCommon{
						msg: msg,
						err: nil,
					})
					if result2.Err() != nil {
						t.Errorf("got a error when testing command: %v", result2.Err())
					}

					res1 := msg + " 1 visited"
					if event1.msg != res1 {
						t.Error("result not match")
					}
					if result2.Value().(string) != (msg + " 1 visited") {
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
		result := mediator.Send(context.TODO(), &TestCommandCommon{
			msg: "adjust test",
		})
		if result.Err() != ants.ErrPoolOverload {
			t.Errorf("want got error %v, but got %v", ants.ErrPoolOverload, result.Err())
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
		result := mediator.Send(context.TODO(), &TestCommandCommon{
			msg: "adjust test",
		})
		if result.Err() != nil {
			t.Errorf("got error %+v", result.Err())
		}
	})

	t.Run("panic test", func(t *testing.T) {
		panicEvent := &PanicEvent{msg: "just panic"}
		result := mediator.Publish(context.TODO(), panicEvent)
		expectErrMsg := "got panic when running *mediator_test.PanicEvent event, cause: " + panicEvent.msg
		if !strings.Contains(result.Err().Error(), expectErrMsg) {
			t.Errorf("error not match, expect: %v, actual : %v", expectErrMsg, result.Err().Error())
		}
	})
}

type (
	ErrEvent struct {
		msg string
	}

	ErrEventHandler struct{}
)

func (e *ErrEvent) Type() reflect.Type {
	return reflect.TypeOf(e)
}

func (h *ErrEventHandler) Handle(ctx context.Context, event INotification) error {
	return errors.New(event.(*ErrEvent).msg)
}

func TestResult(t *testing.T) {
	mediator := New(
		SetRoutinePool(
			new(TestRoutinePool),
		),
	).
		RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
		RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
		RegisterEventHandler(new(ErrEvent).Type(), new(ErrEventHandler)).
		RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
		RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
		Build()

	t.Run("publish event error test", func(t *testing.T) {
		result := mediator.Publish(context.TODO(), &TestEvent1{
			msg: "no error",
		})

		if result.HasError() != false {
			t.Error("expect result.HasError() return false, but got true")
		}

		msg := "this is an error"
		result = mediator.Publish(context.TODO(), &ErrEvent{
			msg: msg,
		})

		if result.HasError() != true {
			t.Error("expect result.HasError() return true, but got false")
		}
		if !strings.Contains(result.Err().Error(), msg) {
			t.Errorf("wrong error message, expect: %v, actual: %v", msg, result.Err().Error())
		}
	})

	t.Run("publish event value test", func(t *testing.T) {
		result := mediator.Publish(context.TODO(), &TestEvent1{
			msg: "no error",
		})

		if result.HasError() != false {
			t.Error("expect result.HasError() return false, but got true")
		}
		if result.HasValue() != false {
			t.Error("expect result.HasValue() return false, but got true")
		}
		if result.Value() != nil {
			t.Errorf("expect result.Value() return nil, but got %v", result.Value())
		}
	})

	t.Run("send command error test", func(t *testing.T) {
		msg := "this is a command error test"
		result := mediator.Send(context.TODO(), &TestCommandCommon{
			msg:      "this is a test",
			duration: 0,
			err:      errors.New(msg),
		})

		if result.HasError() == false {
			t.Error("expect result.HasError() return true, but got false")
		}
		if !strings.Contains(result.Err().Error(), msg) {
			t.Errorf("wrong error message, expect contains: %v, actual massage: %v", msg, result.Err().Error())
		}
	})

	t.Run("send command value test", func(t *testing.T) {
		msg := "this is a test"
		result := mediator.Send(context.TODO(), &TestCommandCommon{
			msg:      msg,
			duration: 0,
			err:      nil,
		})

		if result.HasError() == true {
			t.Error("expect result.HasError() return false, but got true")
		}
		if result.HasValue() != true {
			t.Error("expect result.HasValue() return true, but got false")
		}

		var resString string
		result.ValueT(&resString)
		if resString != result.Value().(string) {
			t.Errorf("value not equal between result.ValueT() -> %v and resul.Value() -> %v", resString, result.Value().(string))
		}
		if !strings.Contains(resString, msg) {
			t.Errorf("wrong value, expect contains: %v, actual value: %v", msg, resString)
		}
	})
}

type (
	MultiErrEvent         struct{}
	MultiErrEventHandler1 struct{}
	MultiErrEventHandler2 struct{}
	MultiErrEventHandler3 struct{}
)

func (e *MultiErrEvent) Type() reflect.Type {
	return reflect.TypeOf(e)
}

func (h *MultiErrEventHandler1) Handle(ctx context.Context, event INotification) error {
	return pkgerr.New("MutilErrEventHandler[1]")
}

func (h *MultiErrEventHandler2) Handle(ctx context.Context, event INotification) error {
	return pkgerr.New("MutilErrEventHandler[2]")
}

func (h *MultiErrEventHandler3) Handle(ctx context.Context, event INotification) error {
	return pkgerr.New("MutilErrEventHandler[3]")
}

func TestErrorNotification(t *testing.T) {
	mediator := New(
		SetRoutinePool(
			new(TestRoutinePool),
		),
	).
		RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
		RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
		RegisterEventHandler(new(ErrEvent).Type(), new(ErrEventHandler)).
		RegisterEventHandler(new(MultiErrEvent).Type(), new(MultiErrEventHandler1)).
		RegisterEventHandler(new(MultiErrEvent).Type(), new(MultiErrEventHandler2)).
		RegisterEventHandler(new(MultiErrEvent).Type(), new(MultiErrEventHandler3)).
		RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
		RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
		Build()

	t.Run("multi error test", func(t *testing.T) {
		result := mediator.Publish(context.TODO(), new(MultiErrEvent))
		if !result.HasError() {
			t.Error("expect an error, but not got")
		}
		errNoti := result.Err().(*ErrorNotification)
		if len(errNoti.Errors()) != 3 {
			t.Errorf("wrong error number, expect: %v, actual: %v", 3, len(errNoti.Errors()))
		}
		t.Logf("TestErrorNotification: the errNoti is: %v\n", errNoti)
	})
}

type (
	CommandForBehavior struct {
		ID             string
		LoggingVisited bool
		PrinterVisited bool
	}
	CommandForBehaviorCommandHandler struct{}
	LoggerCommandBehaviorHandler     struct{}
	PrintCommandBehaviorHandler      struct{}
	ValidationCommandBehaviorHandler struct{}
)

func (c *CommandForBehavior) Type() reflect.Type {
	return reflect.TypeOf(c)
}

func (c *CommandForBehaviorCommandHandler) Handle(ctx context.Context, command IRequest) (interface{}, error) {
	return command.(*CommandForBehavior).ID, nil
}

func (h *LoggerCommandBehaviorHandler) Handle(ctx context.Context, command IRequest, next func(ctx context.Context) IResultContext) IResultContext {
	log.Println("LoggerCommandBehaviorHandler: handler command...")
	cfb, ok := command.(*CommandForBehavior)
	if ok {
		cfb.LoggingVisited = true
	}
	res := next(ctx)
	log.Println("LoggerCommandBehaviorHandler: process done.")
	return res
}

func (h *PrintCommandBehaviorHandler) Handle(ctx context.Context, command IRequest, next func(ctx context.Context) IResultContext) IResultContext {
	log.Printf("PrintCommandBehaviorHandler: command: %v\n", command)
	res := next(ctx)
	cfb, ok := command.(*CommandForBehavior)
	if ok {
		cfb.PrinterVisited = true
	}
	return res
}

func (h *ValidationCommandBehaviorHandler) Handle(ctx context.Context, command IRequest, next func(ctx context.Context) IResultContext) IResultContext {
	cfb, ok := command.(*CommandForBehavior)
	if ok {
		if cfb.ID == "" {
			return (&Result{}).SetErr(errors.New("ID can't be empty"))
		}
	}
	return next(ctx)
}

func TestBehaviors(t *testing.T) {
	mediator := New(
		SetRoutinePool(
			new(TestRoutinePool),
		),
	).
		RegisterBehaviorHandler(new(PrintCommandBehaviorHandler)).
		RegisterBehaviorHandler(new(LoggerCommandBehaviorHandler)).
		RegisterBehaviorHandler(new(ValidationCommandBehaviorHandler)).
		RegisterEventHandler(new(TestEvent1).Type(), new(TestEvent1Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler2)).
		RegisterEventHandler(new(TestEvent2).Type(), new(TestEvent2Handler3)).
		RegisterEventHandler(new(BlockEvent).Type(), new(BlockEventHandler)).
		RegisterEventHandler(new(ErrEvent).Type(), new(ErrEventHandler)).
		RegisterEventHandler(new(MultiErrEvent).Type(), new(MultiErrEventHandler1)).
		RegisterEventHandler(new(MultiErrEvent).Type(), new(MultiErrEventHandler2)).
		RegisterEventHandler(new(MultiErrEvent).Type(), new(MultiErrEventHandler3)).
		RegisterCommandHandler(new(TestCommandCommon).Type(), new(TestCommandCommonHandler)).
		RegisterCommandHandler(new(TestCommand1).Type(), new(TestCommand1Handler)).
		RegisterCommandHandler(new(CommandForBehavior).Type(), new(CommandForBehaviorCommandHandler)).
		Build()

	t.Run("nomal test", func(t *testing.T) {
		msg := "this is test command for behavior testing"
		res := mediator.Send(context.TODO(), &TestCommand1{
			msg: msg,
		})

		if res.HasError() {
			t.Errorf("got a error: %v", res.Err())
		}
		if !strings.Contains(res.Value().(string), msg) {
			t.Errorf("wrong return value, expect: %v, actual: %v", msg, res.Value())
		}
	})

	t.Run("visited test", func(t *testing.T) {
		command := &CommandForBehavior{
			ID:             "id",
			LoggingVisited: false,
			PrinterVisited: false,
		}
		res := mediator.Send(context.TODO(), command)

		if res.HasError() {
			t.Errorf("got a error: %v", res.Err())
		}
		if res.Value().(string) != command.ID {
			t.Errorf("wrong return value, expect: %v, actual: %v", command.ID, res.Value())
		}
		if command.LoggingVisited == false {
			t.Errorf("logging behavior handler not visited the command")
		}
		if command.PrinterVisited == false {
			t.Errorf("printer behavior handler not visited the command")
		}
	})

	t.Run("validation test", func(t *testing.T) {
		command := &CommandForBehavior{
			ID:             "",
			LoggingVisited: false,
			PrinterVisited: false,
		}
		res := mediator.Send(context.TODO(), command)
		if res.HasError() == false {
			t.Errorf("expect got a error when useing empty ID. It should be intercept by validation behavior handler.")
		}
		errMsg := "ID can't be empty"
		if res.Err().Error() != errMsg {
			t.Errorf("wrong error message, expect: %v, actual: %v", errMsg, res.Err().Error())
		}
		if command.LoggingVisited {
			t.Errorf("expect intercept by validation behavior handler but the logging behavior handler has been handled ")
		}
		if command.PrinterVisited {
			t.Errorf("expect intercept by validation behavior handler but the Printer behavior handler has been handled ")
		}
	})
}
