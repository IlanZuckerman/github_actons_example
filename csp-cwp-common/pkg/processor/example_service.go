package processor

import (
	"fmt"
	"sync"
	"time"

	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

//Test service definition
type TestService struct {
	ServiceInterface

	livenessLock  sync.RWMutex //For protecting liveness
	readinessLock sync.RWMutex //For protecting readiness
	heartbeatLock sync.RWMutex //For protecting config updates while getting heartbeat.

	//Local ingress Tap
	tap TapInterface

	//Mapping of sinks to events and query types
	eventSinks map[proto.EventType]SinkInterface
	querySinks map[proto.QueryType]SinkInterface

	//For liveness check
	livenessTimestamp time.Time

	//For readiness
	isReady bool

	//For heartbeat and configuration update information
	heartbeatMsg proto.Heartbeat

	//events channel
	events chan *proto.Event

	//For signaling the Run goroutine to stop from Shutdown call
	stop chan bool

	//For testing purpose
	params *TestServiceParams
}

type TestServiceParams struct {
	//Liveness interval
	LivenessInterval time.Duration
	//List of events to send to another processor
	SendEvents []*proto.Event
	//List of handled events by current processor
	ProcessedEvents []*proto.Event
	//List of queries to send to another service
	SendQueries []*proto.Query
	//List of resolved queries by another service
	QueryResults []*proto.QueryResult
}

func NewTestService(params *TestServiceParams) ServiceInterface {
	s := &TestService{
		eventSinks: make(map[proto.EventType]SinkInterface),
		querySinks: make(map[proto.QueryType]SinkInterface),
		events:     make(chan *proto.Event),
		params:     params,
		stop:       make(chan bool),
	}
	s.tap = NewServiceTap(s, s)
	return s
}

//Get ingress tap
func (ts *TestService) GetTap() TapInterface {
	return ts.tap
}

//Get egress event sink
func (ts *TestService) GetEventSink(eventType proto.EventType) (SinkInterface, error) {
	sink, exists := ts.eventSinks[eventType]
	if !exists {
		return nil, fmt.Errorf("missing sink for event type %s", eventType)
	}
	return sink, nil
}

//Run the service
func (ts *TestService) Run() error {
	if ts.params.LivenessInterval == 0 {
		return fmt.Errorf("provided zero liveness interval")
	}

	go func() {
		ticker := time.NewTicker(ts.params.LivenessInterval)
		defer ticker.Stop()

		ts.setReadiness(true)
		defer ts.setReadiness(false)

		for {
			select {
			case event := <-ts.events:
				//Dummy processing, simply add it to list of processed events
				ts.params.ProcessedEvents = append(ts.params.ProcessedEvents, event)
			case <-ts.stop:
				return
			case <-ticker.C:
				ts.setLiveness()
			default:
				//send next queries and events to sinks
				ts.runNextQuery()
				ts.sendNextEvent()
			}
		}
	}()
	return nil
}

//Shutdown the service
func (ts *TestService) Shutdown() error {
	ts.stop <- true
	return nil
}

//Add sink for Event:
//Should be called during bootstrap when building the processors and relations from a single thread.
func (ts *TestService) AddEventSink(eventType proto.EventType, sink SinkInterface) error {
	if _, exists := ts.eventSinks[eventType]; exists {
		return fmt.Errorf("sink already exists for event type %s", eventType)
	}
	ts.eventSinks[eventType] = sink
	return nil
}

//Add sink for Query:
//Should be called during bootstrap when building the processors and relations from a single thread.
func (ts *TestService) AddQuerySink(queryType proto.QueryType, sink SinkInterface) error {
	if _, exists := ts.querySinks[queryType]; exists {
		return fmt.Errorf("sink already exists for query type %s", queryType)
	}
	ts.querySinks[queryType] = sink
	return nil
}

//Readiness check
func (ts *TestService) IsReady() bool {
	ts.readinessLock.RLock()
	defer ts.readinessLock.RUnlock()

	return ts.isReady
}

//Liveness check
func (ts *TestService) IsAlive(gracePeriod time.Duration) bool {
	ts.livenessLock.RLock()
	defer ts.livenessLock.RUnlock()

	return time.Now().Before(ts.livenessTimestamp.Add(gracePeriod * time.Second))
}

//Heartbeat message:
//The composing struct should add the other information based on the implementation and call
//this method for tracking the config updates.
func (p *TestService) GetHeartbeat() proto.Heartbeat {
	p.heartbeatLock.RLock()
	defer p.heartbeatLock.RUnlock()

	return p.heartbeatMsg
}

//Update configuration and heartbeat message
func (p *TestService) UpdateConfiguration(conf *proto.Configuration) error {
	p.heartbeatLock.Lock()
	defer p.heartbeatLock.Unlock()

	p.heartbeatMsg.ConfigurationUUID = conf.UUID
	p.heartbeatMsg.ConfigurationVersion = conf.Version
	return nil
}

//Event handling method
func (ts *TestService) PushEvent(event *proto.Event) error {
	ts.events <- event
	return nil
}

//Query handling method
func (ts *TestService) RunQuery(query *proto.Query) (*proto.QueryResult, error) {
	//Dummy implementation, simply return the correlating result
	return &proto.QueryResult{
		Type: query.Type,
		UUID: query.UUID,
		Info: &proto.QueryResult_Dummy{
			Dummy: &proto.DummyQueryResult{
				Info: query.GetDummy().Info,
			},
		},
	}, nil
}

//Private method for setting readiness indication internally
func (s *TestService) setReadiness(ready bool) {
	s.readinessLock.Lock()
	defer s.readinessLock.Unlock()

	s.isReady = ready
}

//Private method for updating the livness timestamp internally
func (ts *TestService) setLiveness() {
	ts.livenessLock.Lock()
	defer ts.livenessLock.Unlock()

	ts.livenessTimestamp = time.Now()
}

//Private test method for sending the next event to be processed
func (ts *TestService) sendNextEvent() {
	if len(ts.params.SendEvents) == 0 {
		return
	}
	//pop next event
	event := ts.params.SendEvents[0]
	ts.params.SendEvents[0] = nil
	ts.params.SendEvents = ts.params.SendEvents[1 : len(ts.params.SendEvents)-1]

	//find sink for event and sent to it
	sink, exists := ts.eventSinks[event.Type]
	if exists {
		_ = sink.PushEvent(event)
	}
}

//Private test method for running the next query against another service
func (ts *TestService) runNextQuery() {
	if len(ts.params.SendQueries) == 0 {
		return
	}
	//pop next query
	query := ts.params.SendQueries[0]
	ts.params.SendQueries[0] = nil
	ts.params.SendQueries = ts.params.SendQueries[1 : len(ts.params.SendQueries)-1]

	//find a sink to query and run it on
	sink, exists := ts.querySinks[query.Type]
	if exists {
		result, err := sink.RunQuery(query)
		if err == nil {
			ts.params.QueryResults = append(ts.params.QueryResults, result)
		}
	}
}
