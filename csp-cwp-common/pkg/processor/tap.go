package processor

import (
	"fmt"

	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

//This is the events and queries ingress interface for a processor or a service
type TapInterface interface {
	//Run query against local handler
	RunQuery(query *proto.Query) (*proto.QueryResult, error)
	//Push event to local handler
	PushEvent(event *proto.Event) error

	//Setters
	SetQueryHandler(queryHandler ServiceInterface)
	SetEventHandler(eventHandler ProcessorInterface)
}

//This is the events and queries ingress prototype for a processor or a service
type Tap struct {
	TapInterface
	eventHandler ProcessorInterface
	queryHandler ServiceInterface
}

func NewProcessorTap(eventHandler ProcessorInterface) TapInterface {
	return &Tap{
		eventHandler: eventHandler,
	}
}

func NewServiceTap(eventHandler ProcessorInterface, queryHandler ServiceInterface) TapInterface {
	t := NewProcessorTap(eventHandler)
	t.SetQueryHandler(queryHandler)
	return t
}

func (t *Tap) RunQuery(query *proto.Query) (*proto.QueryResult, error) {
	if t.queryHandler == nil {
		return nil, fmt.Errorf("unitialized query handler")
	}
	return t.queryHandler.RunQuery(query)
}

func (t *Tap) PushEvent(event *proto.Event) error {
	if t.eventHandler == nil {
		return fmt.Errorf("unitialized event handler")
	}
	return t.eventHandler.PushEvent(event)
}

func (t *Tap) SetQueryHandler(queryHandler ServiceInterface) {
	t.queryHandler = queryHandler
}

func (t *Tap) SetEventHandler(eventHandler ProcessorInterface) {
	t.eventHandler = eventHandler
}
