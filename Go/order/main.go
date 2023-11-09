package main

import (
	"fmt"
	"github.com/AT-SmFoYcSNaQ/AT2023/Go/customer/config"
	"time"

	"github.com/AT-SmFoYcSNaQ/AT2023/Go/order/model"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/automanaged"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	"github.com/AT-SmFoYcSNaQ/AT2023/Go/order/messages"
	"github.com/AT-SmFoYcSNaQ/AT2023/Go/order/service"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type OrderActor struct {
	remoting *remote.Remote
	context  *actor.RootContext
	service  *service.OrderService
	system   *actor.ActorSystem
}

func (actor *OrderActor) Receive(context actor.Context) {
	// Handle incoming messages
	switch msg := context.Message().(type) {
	case *messages.ReceiveOrder_Request:
		// Received order from customer
		order := model.Order{
			UserId:         msg.UserId,
			ItemId:         msg.ItemId,
			Quantity:       int(msg.Quantity),
			AccountBalance: msg.AccountBalance,
			PricePerItem:   msg.PricePerItem,
			OrderStatus:    "Pending",
		}
		actor.handleOrderReceived(&order, context.Self()) // Pass the order and self reference
	case *messages.OrderPaymentInfo:
		// Payment response from payment actor
		actor.handlePaymentInfoReceived(msg) // Pass payment status and self reference
	case *messages.EmptyMessage:
	}
}

func (actor *OrderActor) handleOrderReceived(order *model.Order, self *actor.PID) {
	fmt.Println("Received message from customer!")

	orderCreated, err := actor.service.Insert(order)
	if err != nil {
		return
	}

	// Create a message to check availability
	message := &messages.CheckAvailability_Request{
		ItemId:   order.ItemId,
		Quantity: int32(order.Quantity),
		OrderId:  orderCreated,
	}

	inventoryGrain := cluster.GetCluster(actor.system).Get("inventory-1", "inventory-actor")
	responseFuture := actor.context.RequestFuture(inventoryGrain, message, time.Second*10)
	response, err := responseFuture.Result()
	if err != nil {
		panic(err)
	}
	responseValues := response.(*messages.CheckAvailability_Response)
	fmt.Println(responseValues.OrderId)
	msgToSend := messages.CheckAvailability_Response{
		OrderId:     responseValues.OrderId,
		IsAvailable: responseValues.IsAvailable,
		Quantity:    responseValues.Quantity,
		ItemName:    responseValues.ItemName,
		ItemPrice:   responseValues.ItemPrice,
	}
	actor.handleAvailabilityChecked(&msgToSend)

}

func (actor *OrderActor) handleAvailabilityChecked(request *messages.CheckAvailability_Response) {
	fmt.Println("Received message from inventory actor!")

	loadConfig, err := config.LoadConfig("./..")
	if err != nil {
		panic(err)
	}

	// Spawn the notification actor
	spawnResponse, err := actor.remoting.SpawnNamed(loadConfig.ActorNotificationAddress+":"+fmt.Sprint(loadConfig.ActorNotificationPort),
		"notification-actor",
		"notification-actor",
		time.Second*10)
	if err != nil {
		panic(err)
	}

	orderUpdated, err := actor.service.GetById(request.OrderId)
	if err != nil {
		panic(err)
	}
	if request.IsAvailable {
		// Item is available
		orderUpdated.OrderStatus = "Pending"
		_, err = actor.service.Insert(orderUpdated)
		if err != nil {
			return
		}
		actor.context.Send(spawnResponse.Pid, &messages.Notification{
			Message: &messages.Message{
				Content: "Pending",
				Action:  "Pending",
				OrderId: orderUpdated.ID.String(),
			},
			ReceiverId: orderUpdated.UserId,
		})
		actor.prepareOrder(15 * time.Second)
		orderUpdated, err = actor.service.GetById(request.OrderId)
		if err != nil {
			panic(err)
		}
		orderUpdated.OrderStatus = "Prepared"
		_, err = actor.service.Insert(orderUpdated)
		if err != nil {
			return
		}
		actor.context.Send(spawnResponse.Pid, &messages.Notification{
			Message: &messages.Message{
				Content: "Order is prepared!",
				Action:  "Prepared",
				OrderId: orderUpdated.ID.String(),
			},
			ReceiverId: orderUpdated.UserId,
		})
		actor.processPayment(request) // Pass self reference for payment actor
	} else {
		// Item is out of stock
		message := &messages.Notification{
			Message: &messages.Message{
				Content: "Order is out of stock",
				Action:  "OutOfStock",
				OrderId: orderUpdated.ID.String(),
			},
			ReceiverId: orderUpdated.UserId,
		}

		orderUpdated, err := actor.service.GetById(request.OrderId)
		if err != nil {
			panic(err)
		}
		orderUpdated.OrderStatus = "OutOfStock"
		_, err = actor.service.Insert(orderUpdated)
		if err != nil {
			return
		}

		actor.context.Send(spawnResponse.Pid, message)
	}
}

func (actor *OrderActor) handlePaymentInfoReceived(request *messages.OrderPaymentInfo) {
	fmt.Println("Received message from payment actor!")

	loadConfig, err := config.LoadConfig("./..")
	if err != nil {
		panic(err)
	}

	// Spawn the notification actor
	spawnResponse, err := actor.remoting.SpawnNamed(
		loadConfig.ActorNotificationAddress+":"+fmt.Sprint(loadConfig.ActorNotificationPort),
		"notification-actor",
		"notification-actor",
		time.Second)
	if err != nil {
		panic(err)
	}

	status := "PaymentFailed"
	if request.IsSuccessful {
		status = "Payment"
	}

	orderUpdated, err := actor.service.GetById(request.OrderId)
	if err != nil {
		panic(err)
	}
	orderUpdated.OrderStatus = status
	_, err = actor.service.Insert(orderUpdated)
	if err != nil {
		return
	}

	actor.context.Send(spawnResponse.Pid, &messages.Notification{
		Message: &messages.Message{
			Content: status,
			Action:  status,
			OrderId: "dasd2",
		},
		ReceiverId: orderUpdated.UserId,
	})
}

func (actor *OrderActor) prepareOrder(seconds time.Duration) {
	fmt.Println("Order preparing process in progress!")
	time.Sleep(seconds)
	fmt.Println("Order preparing process done!")
}

func (actor *OrderActor) processPayment(request *messages.CheckAvailability_Response) {
	// Spawn the payment actor
	loadConfig, err := config.LoadConfig("./..")
	if err != nil {
		panic(err)
	}

	spawnResponse, err := actor.remoting.SpawnNamed(loadConfig.ActorPaymentAddress+":"+fmt.Sprint(loadConfig.ActorPaymentPort),
		"payment-actor",
		"payment-actor",
		5*time.Second)
	if err != nil {
		panic(err)
	}

	order, err := actor.service.GetById(request.OrderId)
	if err != nil {
		panic(err)
	}

	message := &messages.PaymentReq{
		Quantity:       int32(order.Quantity),
		PricePerItem:   request.ItemPrice,
		OrderId:        request.OrderId,
		AccountBalance: float32(order.AccountBalance),
		UserId:         order.UserId,
	}
	actor.context.Send(spawnResponse.Pid, message)
}

func main() {

	system := actor.NewActorSystem()
	orderService := service.CreateOrderService()

	loadConfig, err := config.LoadConfig("./..")
	if err != nil {
		panic(err)
	}

	// Configure and start remote communication with actors
	remoteConfig := remote.Configure(loadConfig.ActorOrderAddress, loadConfig.ActorOrderPort)
	//remoting := remote.NewRemote(system, remoteConfig)

	//remoting.Start()

	// Configure cluster on top of the above remote env
	// This member uses port 6330 for cluster provider, and add ponger member -- localhost:6331 -- as member.
	// With automanaged implementation, one must list up all known members at first place to ping each other.
	// Note that this member itself is not registered as a member member because this only works as a client.
	lookup := disthash.New()
	cp := automanaged.NewWithConfig(10*time.Second,
		6330,
		loadConfig.ActorInventoryAddress+":"+fmt.Sprint(loadConfig.ActorInventoryPort),
		loadConfig.ActorInventoryAddress+":"+fmt.Sprint(loadConfig.ActorInventoryPort+1),
		loadConfig.ActorInventoryAddress+":"+fmt.Sprint(loadConfig.ActorInventoryPort+2))
	clusterConfig := cluster.Configure("cluster-inventory", cp, lookup, remoteConfig)
	c := cluster.New(system, clusterConfig)
	// Start as a client, not as a cluster member.
	c.StartClient()

	// Get the root context of the actor system
	context := system.Root

	// Create the order actor and register it with the remote system
	orderActorProps := actor.PropsFromProducer(func() actor.Actor {
		return &OrderActor{remoting: c.Remote, context: context, service: orderService, system: system}
	})
	c.Remote.Register("order-actor", orderActorProps)

	console.ReadLine()
}
