package processor

import (
	"time"

	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

//Definition of Processor interface
//This is the basic definition for any internal component processing
//and sending events within the agent.
type ProcessorInterface interface {
	//Get Ingress Tap
	GetTap() TapInterface

	//Handle recieved event
	PushEvent(event *proto.Event) error

	//Run the processor
	Run() error

	//Shutdown the processor
	Shutdown() error

	//Add egress sink for event type:
	//Should be called during bootstrap when building the processors and relations.
	//Possbile implementation is to add the sink to the map entry for the given type.
	AddEventSink(eventType proto.EventType, sink SinkInterface) error

	//Add egress sink for Query:
	//Should be called during bootstrap when building the processors and relations.
	//Possbile implementation is to add the sink to the map entry for the given type.
	AddQuerySink(queryType proto.QueryType, sink SinkInterface) error

	//Following are inquiry methods which can be called on each and every processor
	//instance directly and have to access struct members under RWLock as they can be
	//updated and read from different goroutines:

	//Check readiness indication
	IsReady() bool

	//Get liveness indication:
	//gracePeriod serves as a time duration for which liveness can be detrmined by checking if:
	//(now + gracePeriod) <= liveness_timestamp
	IsAlive(gracePeriod time.Duration) bool

	//Get heartbeat message
	GetHeartbeat() proto.Heartbeat

	//Update configuration
	UpdateConfiguration(conf *proto.Configuration) error
}
