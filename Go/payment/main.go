package main

import (
	"fmt"
	"github.com/AT-SmFoYcSNaQ/AT2023/Go/customer/config"
	messages "github.com/AT-SmFoYcSNaQ/AT2023/Go/payment/messages/proto"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"time"
)

type PaymentReq struct {
	Quantity       int32
	PricePerItem   float32
	OrderId        string
	AccountNumber  string
	AccountBalance float32
	UserId         string
}

type PaymentActor struct {
	remoting *remote.Remote
	context  *actor.RootContext
}

func (actor *PaymentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.PaymentReq:
		paymentReq := PaymentReq{
			Quantity:       msg.Quantity,
			PricePerItem:   msg.PricePerItem,
			OrderId:        msg.OrderId,
			AccountBalance: msg.AccountBalance,
			AccountNumber:  msg.AccountNumber,
			UserId:         msg.UserId,
		}
		actor.handlePaymentRequest(paymentReq, context.Self())
	}
}

func (actor *PaymentActor) handlePaymentRequest(paymentReq PaymentReq, self *actor.PID) {
	fmt.Println("Received payment request")
	totalPrice := paymentReq.PricePerItem * float32(paymentReq.Quantity)
	paymentSuccessful := totalPrice <= paymentReq.AccountBalance

	if !paymentSuccessful {
		actor.sendPaymentInfo(paymentReq, paymentSuccessful)
		actor.sendPaymentInfoNotification(paymentReq, paymentSuccessful, self)
		return
	}
	paymentReq.AccountBalance -= totalPrice

	actor.sendPaymentInfo(paymentReq, paymentSuccessful)
	actor.sendPaymentInfoNotification(paymentReq, paymentSuccessful, self)
}

func (actor *PaymentActor) sendPaymentInfo(paymentReq PaymentReq, isSuccessful bool) {
	fmt.Println("Sending payment info to order actor")
	message := &messages.OrderPaymentInfo{
		OrderId:        paymentReq.OrderId,
		IsSuccessful:   isSuccessful,
		AccountNumber:  paymentReq.AccountNumber,
		AccountBalance: paymentReq.AccountBalance,
	}

	loadConfig, err := config.LoadConfig("./..")
	if err != nil {
		panic(err)
	}

	spawnResponse, err := actor.remoting.SpawnNamed(loadConfig.ActorOrderAddress+":"+fmt.Sprint(loadConfig.ActorOrderPort),
		"order-actor",
		"order-actor",
		time.Second)

	if err != nil {
		panic(err)
	}

	actor.context.Send(spawnResponse.Pid, message)
	fmt.Println("Sent payment message to order actor!")
}

func (actor *PaymentActor) sendPaymentInfoNotification(paymentReq PaymentReq, isSuccessful bool, self *actor.PID) {
	fmt.Println("Sending payment info to notification actor")

	paymentMessage := ""

	if isSuccessful {
		paymentMessage = fmt.Sprintf("Payment for orderId %s was successful! New account balance is %.2f", paymentReq.OrderId, paymentReq.AccountBalance)
	} else {
		paymentMessage = fmt.Sprintf("Payment for orderId %s was not successful,account balance did not change.", paymentReq.OrderId)
	}

	loadConfig, err := config.LoadConfig("./..")
	if err != nil {
		panic(err)
	}

	spawnResponse, err := actor.remoting.SpawnNamed(loadConfig.ActorNotificationAddress+":"+fmt.Sprint(loadConfig.ActorNotificationPort),
		"notification-actor",
		"notification-actor",
		time.Second)

	messageContent := &messages.Message{
		Content: paymentMessage,
		Action:  "",
		OrderId: "",
	}

	message := &messages.Notification{
		Message:    messageContent,
		ReceiverId: paymentReq.UserId,
	}

	if err != nil {
		panic(err)
	}

	actor.context.Send(spawnResponse.Pid, message)
	fmt.Println("Sent payment message to notification actor!")
}

func main() {

	system := actor.NewActorSystem()
	loadConfig, err := config.LoadConfig("./..")
	if err != nil {
		panic(err)
	}

	// Configure and start remote communication
	remoteConfig := remote.Configure(loadConfig.ActorPaymentAddress, loadConfig.ActorPaymentPort)
	remoting := remote.NewRemote(system, remoteConfig)

	remoting.Start()

	// Get the root context of the actor system
	context := system.Root

	// Create the payment actor and register it with the remote system
	paymentActorProps := actor.PropsFromProducer(func() actor.Actor { return &PaymentActor{remoting: remoting, context: context} })
	//context.Spawn(paymentActorProps)

	remoting.Register("payment-actor", paymentActorProps)

	console.ReadLine()
}
