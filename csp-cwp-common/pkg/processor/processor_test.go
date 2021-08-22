package processor

import (
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/util/wait"

	pb "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

type ProcessorTestSuite struct {
	suite.Suite
}

func (suite *ProcessorTestSuite) SetupTest() {
}

func (suite *ProcessorTestSuite) TearDownTest() {
}

func (suite *ProcessorTestSuite) TestProcessor__PushEvents() {
	events := prepareEvents(10)

	senderParams := &TestProcessorParams{
		LivenessInterval: time.Second,
		SendEvents:       events,
	}
	//save for later validation
	expectedEvents := make([]*pb.Event, len(events))
	copy(expectedEvents, events)

	receiverParams := &TestProcessorParams{
		LivenessInterval: time.Second,
	}

	//Create events sender processor
	sender := NewTestProcessor(senderParams)

	//Create events receiver processor
	receiver := NewTestProcessor(receiverParams)

	//Connect sender to receiver by a sink to the reciever tap
	sink := NewSink(receiver.GetTap())
	err := sender.AddEventSink(pb.EventType_DummyEventType, sink)
	require.NoError(suite.T(), err, "failed to add  event sink: %s", err)

	//Run both ends
	err = receiver.Run()
	require.NoError(suite.T(), err, "failed to run reciever: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return receiver.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to run reciever: %s", err)

	err = sender.Run()
	require.NoError(suite.T(), err, "failed to run sender: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return sender.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to run sender: %s", err)

	//Wait for all events to arrive to receiver processor
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return len(expectedEvents) == len(receiverParams.ProcessedEvents), nil })
	require.NoError(suite.T(), err, "failed to process all events: %s", err)

	//Compare expected to processed events information
	for i, e := range receiverParams.ProcessedEvents {
		expected := proto.MarshalTextString(expectedEvents[i])
		processed := proto.MarshalTextString(e)
		require.Equal(suite.T(), processed, expected, "mismatching events")
	}

	//Shutdown the reciever
	err = receiver.Shutdown()
	require.NoError(suite.T(), err, "failed to shutdown reciver: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return !receiver.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to shutdown reciever: %s", err)

	//Shutdown the sender
	err = sender.Shutdown()
	require.NoError(suite.T(), err, "failed to shutdown sender: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return !sender.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to shutdown sender: %s", err)
}

func (suite *ProcessorTestSuite) TestProcessor__RunServiceQueries() {
	queries, expectedResults := prepareQueries(10)

	senderParams := &TestProcessorParams{
		LivenessInterval: time.Second,
		SendQueries:      queries,
	}

	receiverParams := &TestServiceParams{
		LivenessInterval: time.Second,
	}

	//Create events sender processor
	sender := NewTestProcessor(senderParams)

	//Create events receiver processor
	receiver := NewTestService(receiverParams)

	//Connect sender to receiver by a sink to the reciever tap
	sink := NewSink(receiver.GetTap())
	err := sender.AddQuerySink(pb.QueryType_DummyQueryType, sink)
	require.NoError(suite.T(), err, "failed to add  event sink: %s", err)

	//Run both ends
	err = receiver.Run()
	require.NoError(suite.T(), err, "failed to run reciever: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return receiver.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to run reciever: %s", err)

	err = sender.Run()
	require.NoError(suite.T(), err, "failed to run sender: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return sender.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to run sender: %s", err)

	//Wait for queries to be run and have results at the sender
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return len(expectedResults) == len(senderParams.QueryResults), nil })
	require.NoError(suite.T(), err, "failed to run all queries: %s", err)

	//Compare expected to processed events information
	for i, r := range senderParams.QueryResults {
		expected := proto.MarshalTextString(expectedResults[i])
		result := proto.MarshalTextString(r)
		require.Equal(suite.T(), result, expected, "mismatching query results")
	}

	//Shutdown the reciever
	err = receiver.Shutdown()
	require.NoError(suite.T(), err, "failed to shutdown reciver: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return !receiver.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to shutdown reciever: %s", err)

	//Shutdown the sender
	err = sender.Shutdown()
	require.NoError(suite.T(), err, "failed to shutdown sender: %s", err)
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) { return !sender.IsReady(), nil })
	require.NoError(suite.T(), err, "failed to shutdown sender: %s", err)
}

func (suite *ProcessorTestSuite) TestProcessor__UpdateConfiguration() {
	p := NewTestProcessor(&TestProcessorParams{
		LivenessInterval: time.Second,
	})
	//Send Configuration update to processor
	conf := &pb.Configuration{
		UUID:    "configuraion-uuid",
		Version: 1,
		Info:    "configuration information",
	}
	err := p.UpdateConfiguration(conf)
	require.NoError(suite.T(), err, "failed to update configuration: %s", err)

	//Get heartbeat information of updated configuration UUID and version
	heartbeat := p.GetHeartbeat()
	require.Equal(suite.T(), heartbeat.ConfigurationUUID, conf.UUID, "mismtaching config UUID %s %s", heartbeat.ConfigurationUUID, conf.UUID)
	require.Equal(suite.T(), heartbeat.ConfigurationVersion, conf.Version, "mismtaching config Version %s %s", heartbeat.ConfigurationVersion, conf.Version)
}

func (suite *ProcessorTestSuite) TestProcessor__Liveness() {
	LivenessInterval := 100 * time.Millisecond
	p := NewTestProcessor(&TestProcessorParams{
		LivenessInterval: LivenessInterval,
	})
	require.False(suite.T(), p.IsAlive(LivenessInterval), "processor is marked alive before running")
	err := p.Run()
	require.NoError(suite.T(), err, "failed to run processor: %s", err)

	//Check there is liveness indication within the interval
	err = wait.Poll(10*time.Millisecond, 2*LivenessInterval, func() (bool, error) { return p.IsAlive(LivenessInterval), nil })
	require.NoError(suite.T(), err, "running processor is not marked alive within interval limit: %s", err)

	//Check there is no liveness indication in between intervals
	require.False(suite.T(), p.IsAlive(0), "running processor should not be marked alive before next interval")

	//Shutdown the processor
	err = p.Shutdown()
	require.NoError(suite.T(), err, "failed to shutdown processor: %s", err)

	//Livenss timestamp should stop updating
	err = wait.Poll(10*time.Millisecond, 2*LivenessInterval, func() (bool, error) { return p.IsAlive(0), nil })
	require.Error(suite.T(), err, "failed to stop liveness updates after shutdown: %s", err)
}

func TestProcessor__RUN(t *testing.T) {
	crt := new(ProcessorTestSuite)
	suite.Run(t, crt)
}

//Helper functions

func prepareEvents(num int) []*pb.Event {
	events := make([]*pb.Event, 0)

	for i := 0; i < num; i++ {
		event := &pb.Event{
			Type: pb.EventType_DummyEventType,
			Info: &pb.Event_Dummy{
				Dummy: &pb.DummyEvent{
					Info: "Event " + strconv.Itoa(i),
				},
			},
		}
		events = append(events, event)
	}
	return events
}

func prepareQueries(num int) ([]*pb.Query, []*pb.QueryResult) {
	queries := make([]*pb.Query, 0)
	expectedResults := make([]*pb.QueryResult, 0)

	for i := 0; i < num; i++ {
		query := &pb.Query{
			Type: pb.QueryType_DummyQueryType,
			UUID: "query-uuid",
			Info: &pb.Query_Dummy{
				Dummy: &pb.DummyQuery{
					Info: "Query " + strconv.Itoa(i),
				},
			},
		}
		queries = append(queries, query)

		expectedResult := &pb.QueryResult{
			Type: query.Type,
			UUID: query.UUID,
			Info: &pb.QueryResult_Dummy{
				Dummy: &pb.DummyQueryResult{
					Info: "Query " + strconv.Itoa(i),
				},
			},
		}
		expectedResults = append(expectedResults, expectedResult)
	}
	return queries, expectedResults
}
