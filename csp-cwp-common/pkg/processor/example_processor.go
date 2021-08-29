package processor

import (
	"fmt"
	"sync"
	"time"

	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

//Test processor definition
type TestProcessor struct {
	ProcessorInterface

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
	params *TestProcessorParams
}

type TestProcessorParams struct {
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

func NewTestProcessor(params *TestProcessorParams) ProcessorInterface {
	p := &TestProcessor{
		eventSinks: make(map[proto.EventType]SinkInterface),
		querySinks: make(map[proto.QueryType]SinkInterface),
		events:     make(chan *proto.Event),
		params:     params,
		stop:       make(chan bool),
	}
	p.tap = NewProcessorTap(p)
	return p
}

//Get ingress tap
func (tp *TestProcessor) GetTap() TapInterface {
	return tp.tap
}

//Run the service
func (tp *TestProcessor) Run() error {
	if tp.params.LivenessInterval == 0 {
		return fmt.Errorf("provided zero liveness interval")
	}

	go func() {
		ticker := time.NewTicker(tp.params.LivenessInterval)
		defer ticker.Stop()

		tp.setReadiness(true)
		defer tp.setReadiness(false)

		for {
			select {
			case event := <-tp.events:
				//Dummy processing, simply add it to list of processed events
				tp.params.ProcessedEvents = append(tp.params.ProcessedEvents, event)
			case <-tp.stop:
				return
			case <-ticker.C:
				tp.setLiveness()
			default:
				//send next queries and events to sinks
				tp.runNextQuery()
				tp.sendNextEvent()
			}
		}
	}()
	return nil
}

//Shutdown the processor
func (tp *TestProcessor) Shutdown() error {
	tp.stop <- true
	return nil
}

//Add sink for Event:
//Should be called during bootstrap when building the processors and relations from a single thread.
func (tp *TestProcessor) AddEventSink(eventType proto.EventType, sink SinkInterface) error {
	if _, exists := tp.eventSinks[eventType]; exists {
		return fmt.Errorf("sink already exists for event type %s", eventType)
	}
	tp.eventSinks[eventType] = sink
	return nil
}

//Add sink for Query:
//Should be called during bootstrap when building the processors and relations from a single thread.
func (tp *TestProcessor) AddQuerySink(queryType proto.QueryType, sink SinkInterface) error {
	if _, exists := tp.querySinks[queryType]; exists {
		return fmt.Errorf("sink already exists for query type %s", queryType)
	}
	tp.querySinks[queryType] = sink
	return nil
}

//Readiness check
func (tp *TestProcessor) IsReady() bool {
	tp.readinessLock.RLock()
	defer tp.readinessLock.RUnlock()

	return tp.isReady
}

//Liveness check
func (tp *TestProcessor) IsAlive(gracePeriod time.Duration) bool {
	tp.livenessLock.RLock()
	defer tp.livenessLock.RUnlock()

	return time.Now().Before(tp.livenessTimestamp.Add(gracePeriod * time.Second))
}

//Heartbeat message:
//The composing struct should add the other information based on the implementation and call
//this method for tracking the config updates.
func (tp *TestProcessor) GetHeartbeat() proto.Heartbeat {
	tp.heartbeatLock.RLock()
	defer tp.heartbeatLock.RUnlock()

	return tp.heartbeatMsg
}

//Update configuration and heartbeat message
func (tp *TestProcessor) UpdateConfiguration(conf *proto.Configuration) error {
	tp.heartbeatLock.Lock()
	defer tp.heartbeatLock.Unlock()

	tp.heartbeatMsg.ConfigurationUUID = conf.UUID
	tp.heartbeatMsg.ConfigurationVersion = conf.Version
	return nil
}

//Event handling method
func (tp *TestProcessor) PushEvent(event *proto.Event) error {
	tp.events <- event
	return nil
}

//Private method for setting readiness indication internally
func (tp *TestProcessor) setReadiness(ready bool) {
	tp.readinessLock.Lock()
	defer tp.readinessLock.Unlock()

	tp.isReady = ready
}

//Private method for updating the livness timestamp internally
func (tp *TestProcessor) setLiveness() {
	tp.livenessLock.Lock()
	defer tp.livenessLock.Unlock()

	tp.livenessTimestamp = time.Now()
}

//Private test method for sending the next event to be processed
func (tp *TestProcessor) sendNextEvent() {
	if len(tp.params.SendEvents) == 0 {
		return
	}
	//pop next event
	event := tp.params.SendEvents[0]
	tp.params.SendEvents[0] = nil
	tp.params.SendEvents = tp.params.SendEvents[1:]

	//find sink for event and sent to it
	sink, exists := tp.eventSinks[event.Type]
	if exists {
		_ = sink.PushEvent(event)
	}
}

//Private test method for running the next query against another service
func (tp *TestProcessor) runNextQuery() {
	if len(tp.params.SendQueries) == 0 {
		return
	}
	//pop next query
	query := tp.params.SendQueries[0]
	tp.params.SendQueries[0] = nil
	tp.params.SendQueries = tp.params.SendQueries[1:]

	//find a sink to query and run it on
	sink, exists := tp.querySinks[query.Type]
	if exists {
		result, err := sink.RunQuery(query)
		if err == nil {
			tp.params.QueryResults = append(tp.params.QueryResults, result)
		}
	}
}
