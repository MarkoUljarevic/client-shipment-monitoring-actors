// Package messages is generated by protoactor-go/protoc-gen-gograin@0.1.0
package messages

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	logmod "github.com/asynkron/protoactor-go/log"
	"google.golang.org/protobuf/proto"
)

var (
	plog = logmod.New(logmod.InfoLevel, "[GRAIN][messages]")
	_    = proto.Marshal
	_    = fmt.Errorf
	_    = math.Inf
)

// SetLogLevel sets the log level.
func SetLogLevel(level logmod.Level) {
	plog.SetLevel(level)
}

var xInventoryActorFactory func() InventoryActor

// InventoryActorFactory produces a InventoryActor
func InventoryActorFactory(factory func() InventoryActor) {
	xInventoryActorFactory = factory
}

// GetInventoryActorGrainClient instantiates a new InventoryActorGrainClient with given Identity
func GetInventoryActorGrainClient(c *cluster.Cluster, id string) *InventoryActorGrainClient {
	if c == nil {
		panic(fmt.Errorf("nil cluster instance"))
	}
	if id == "" {
		panic(fmt.Errorf("empty id"))
	}
	return &InventoryActorGrainClient{Identity: id, cluster: c}
}

// GetInventoryActorKind instantiates a new cluster.Kind for InventoryActor
func GetInventoryActorKind(opts ...actor.PropsOption) *cluster.Kind {
	props := actor.PropsFromProducer(func() actor.Actor {
		return &InventoryActorActor{
			Timeout: 60 * time.Second,
		}
	}, opts...)
	kind := cluster.NewKind("InventoryActor", props)
	return kind
}

// GetInventoryActorKind instantiates a new cluster.Kind for InventoryActor
func NewInventoryActorKind(factory func() InventoryActor, timeout time.Duration, opts ...actor.PropsOption) *cluster.Kind {
	xInventoryActorFactory = factory
	props := actor.PropsFromProducer(func() actor.Actor {
		return &InventoryActorActor{
			Timeout: timeout,
		}
	}, opts...)
	kind := cluster.NewKind("InventoryActor", props)
	return kind
}

// InventoryActor interfaces the services available to the InventoryActor
type InventoryActor interface {
	Init(ctx cluster.GrainContext)
	Terminate(ctx cluster.GrainContext)
	ReceiveDefault(ctx cluster.GrainContext)
	CheckAvailability(*CheckAvailability_Request, cluster.GrainContext) (*CheckAvailability_Response, error)
}

// InventoryActorGrainClient holds the base data for the InventoryActorGrain
type InventoryActorGrainClient struct {
	Identity string
	cluster  *cluster.Cluster
}

// CheckAvailability requests the execution on to the cluster with CallOptions
func (g *InventoryActorGrainClient) CheckAvailability(r *CheckAvailability_Request, opts ...cluster.GrainCallOption) (*CheckAvailability_Response, error) {
	bytes, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}
	reqMsg := &cluster.GrainRequest{MethodIndex: 0, MessageData: bytes}
	resp, err := g.cluster.Call(g.Identity, "InventoryActor", reqMsg, opts...)
	if err != nil {
		return nil, err
	}
	switch msg := resp.(type) {
	case *cluster.GrainResponse:
		result := &CheckAvailability_Response{}
		err = proto.Unmarshal(msg.MessageData, result)
		if err != nil {
			return nil, err
		}
		return result, nil
	case *cluster.GrainErrorResponse:
		return nil, errors.New(msg.Err)
	default:
		return nil, errors.New("unknown response")
	}
}

// InventoryActorActor represents the actor structure
type InventoryActorActor struct {
	ctx     cluster.GrainContext
	inner   InventoryActor
	Timeout time.Duration
}

// Receive ensures the lifecycle of the actor for the received message
func (a *InventoryActorActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started: //pass
	case *cluster.ClusterInit:
		a.ctx = cluster.NewGrainContext(ctx, msg.Identity, msg.Cluster)
		a.inner = xInventoryActorFactory()
		a.inner.Init(a.ctx)

		if a.Timeout > 0 {
			ctx.SetReceiveTimeout(a.Timeout)
		}
	case *actor.ReceiveTimeout:
		ctx.Poison(ctx.Self())
	case *actor.Stopped:
		a.inner.Terminate(a.ctx)
	case actor.AutoReceiveMessage: // pass
	case actor.SystemMessage: // pass

	case *cluster.GrainRequest:
		switch msg.MethodIndex {
		case 0:
			req := &CheckAvailability_Request{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				plog.Error("CheckAvailability(CheckAvailability_Request) proto.Unmarshal failed.", logmod.Error(err))
				resp := &cluster.GrainErrorResponse{Err: err.Error()}
				ctx.Respond(resp)
				return
			}
			r0, err := a.inner.CheckAvailability(req, a.ctx)
			if err != nil {
				resp := &cluster.GrainErrorResponse{Err: err.Error()}
				ctx.Respond(resp)
				return
			}
			bytes, err := proto.Marshal(r0)
			if err != nil {
				plog.Error("CheckAvailability(CheckAvailability_Request) proto.Marshal failed", logmod.Error(err))
				resp := &cluster.GrainErrorResponse{Err: err.Error()}
				ctx.Respond(resp)
				return
			}
			resp := &cluster.GrainResponse{MessageData: bytes}
			ctx.Respond(resp)

		}
	default:
		a.inner.ReceiveDefault(a.ctx)
	}
}
