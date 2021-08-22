package processor

import (
	"fmt"

	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

//TODO: Consider splitting to QuerySink and EventSink
type SinkInterface interface {
	//Run query against localTap or client to remote service
	RunQuery(query *proto.Query) (*proto.QueryResult, error)
	//Push event to localTap or client to remote processor
	PushEvent(event *proto.Event) error
}

//This is the events and queries egress object for a processor or a service
type Sink struct {
	SinkInterface
	tap TapInterface
}

//Create sink to a local Tap
func NewSink(tap TapInterface) SinkInterface {
	s := &Sink{
		tap: tap,
	}
	return s
}

func (s *Sink) RunQuery(query *proto.Query) (*proto.QueryResult, error) {
	if s.tap != nil {
		return s.tap.RunQuery(query)
	}
	return nil, fmt.Errorf("no valid tap")
}

func (s *Sink) PushEvent(event *proto.Event) error {
	if s.tap != nil {
		return s.tap.PushEvent(event)
	}
	return fmt.Errorf("no valid tap")
}
