package mediator_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	mediator "github.com/liujh2010/mediator"
)

/*
	EVENTS
*/

var (
	_ mediator.INotification        = (*CreatedOrderEvent)(nil)
	_ mediator.INotificationHandler = (*OrderCreatedEmailSenderHandler)(nil)
	_ mediator.INotificationHandler = (*InventorySystemNoticeHandler)(nil)
)

type CreatedOrderEvent struct {
	orderID       string
	orderName     string
	price         float64
	count         int
	customerEmail string
}

func (e *CreatedOrderEvent) Type() reflect.Type {
	return reflect.TypeOf(e)
}

type OrderCreatedEmailSenderHandler struct {
	emailServer *EmailServer
}

func (h *OrderCreatedEmailSenderHandler) Handle(ctx context.Context, event mediator.INotification) error {
	createdOrderEvent := event.(*CreatedOrderEvent)
	return h.emailServer.SendEmail(createdOrderEvent.customerEmail, fmt.Sprint(createdOrderEvent))
}

type InventorySystemNoticeHandler struct {
	inventorySystemAPI *InventorySystem
}

func (h *InventorySystemNoticeHandler) Handle(ctx context.Context, event mediator.INotification) error {
	createdOrderEvent := event.(*CreatedOrderEvent)
	return h.inventorySystemAPI.DeductingInventory(createdOrderEvent.orderID, createdOrderEvent.count)
}

/*
	COMMAND
*/

var (
	_ mediator.IRequest        = (*CreateOrderCommand)(nil)
	_ mediator.IRequestHandler = (*CreateOrderCommandHandler)(nil)
)

type CreateOrderCommand struct {
	orderID    string
	userID     string
	orderName  string
	count      int
	totalPrice float64
}

func (c *CreateOrderCommand) Type() reflect.Type {
	return reflect.TypeOf(c)
}

type CreateOrderCommandHandler struct{}

func (h *CreateOrderCommandHandler) Handle(ctx context.Context, command mediator.IRequest) (interface{}, error) {
	createOrderCommand := command.(*CreateOrderCommand)

	/*
	*
	*	do some business logic...
	*
	 */

	fmt.Printf("the order %v was created\n", createOrderCommand.orderID)
	return createOrderCommand.orderID, nil
}

func TestExample(t *testing.T) {
	// Notice: the mediator should initialize on the stage of application start.
	// This code just for easy to demo.
	mediator := mediator.New().

		// register two event handler for "CreatedOrderEvent" to the mediator
		RegisterEventHandler(new(CreatedOrderEvent).Type(), &OrderCreatedEmailSenderHandler{
			emailServer: new(EmailServer),
		}).
		RegisterEventHandler(new(CreatedOrderEvent).Type(), &InventorySystemNoticeHandler{
			inventorySystemAPI: new(InventorySystem),
		}).

		// register command handler for "CreateOrderCommand" to the mediator
		RegisterCommandHandler(new(CreateOrderCommand).Type(), &CreateOrderCommandHandler{}).

		// call Build() to finish the stage of register
		Build()

	// build a create order command to trigger command handler
	createOrderCommand := &CreateOrderCommand{
		orderID:    "9e12d851-fe4c-4dd5-a73d-9cead7df91f4",
		userID:     "customer@demo.com",
		orderName:  "foo",
		count:      10,
		totalPrice: 198.23,
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Second*20)

	// In this case, the mediator will find out the handler of "CreateOrderCommand" to handle the command.
	orderID, err := mediator.Send(ctx, createOrderCommand) // trigger the command

	// check the error and result
	if err != nil {
		t.Errorf("got an unexpected error: %v", err)
	}
	if orderID.(string) != createOrderCommand.orderID {
		t.Errorf("wrong order_id, want: %v, got: %v", createOrderCommand.orderID, orderID)
	}

	// build the created order event
	event := &CreatedOrderEvent{
		orderID:       createOrderCommand.orderID,
		orderName:     createOrderCommand.orderName,
		price:         createOrderCommand.totalPrice,
		count:         createOrderCommand.count,
		customerEmail: createOrderCommand.userID,
	}

	// Publish the "CreatedOrderEvent" when create order command is finished.
	// The publish action will be trigger two event handlers that register by the above code,
	// these two handlers will be concurrent processing the event, the process will not be interrupted,
	// even if one of them has an error. However the cancellation via context is still supported.
	eventErr := mediator.Publish(ctx, event)

	// check the error
	if eventErr != nil {
		t.Errorf("got an unexpected error: %v", eventErr)
	}
}

type EmailServer struct{}

func (s *EmailServer) SendEmail(email string, content string) error {
	fmt.Printf("the email sent to %v\n", email)
	return nil
}

type InventorySystem struct{}

func (s *InventorySystem) DeductingInventory(productID string, count int) error {
	fmt.Printf("the deducting inventory succeed by product_id: %v, with count: %v\n", productID, count)
	return nil
}
