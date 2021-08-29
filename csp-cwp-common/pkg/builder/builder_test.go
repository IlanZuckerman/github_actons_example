package builder

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rapid7/csp-cwp-common/pkg/processor"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/util/wait"
)

type BuilderTestSuite struct {
	suite.Suite
}

func (suite *BuilderTestSuite) SetupTest() {
}

func (suite *BuilderTestSuite) TearDownTest() {
}

func (suite *BuilderTestSuite) TestBuilder__MissingConstructor() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	errors := builder.Run()
	require.NotZero(suite.T(), len(errors), "builder was able run with processor missing a constructor")

	//Check that no processor reference was added to instances map
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 0, processors)
}

func (suite *BuilderTestSuite) TestBuilder__DuplicateTypeConstructor() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//First addition of Type1 constructor should pass
	err = builder.AddConstructor("Type1", func() {})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Second addition of Type1 constructor should fail
	err = builder.AddConstructor("Type1", func() {})
	require.Error(suite.T(), err, "added constructor twice for same processor type")
}

func (suite *BuilderTestSuite) TestBuilder__ErrorConstructor() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Add constuctor which returns an error.
	errorConstructor := func() (processor.ProcessorInterface, error) {
		return nil, fmt.Errorf("test error")
	}
	err = builder.AddConstructor("Type1", errorConstructor)
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Try to build and run the mesh.
	errors := builder.Run()
	require.NotZero(suite.T(), len(errors), "builder was able to run with error on constructor")

	//Check that the processor reference was not added to instances map.
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 0, processors)
}

func (suite *BuilderTestSuite) TestBuilder__NonServiceInQueryDestination() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- source: Instance1
  destination: Instance2
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Instance1 is a Processor type
	err = builder.AddConstructor("Type1", processor.NewTestProcessor, &processor.TestProcessorParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Instance2 is also Processor type
	err = builder.AddConstructor("Type2", processor.NewTestProcessor, &processor.TestProcessorParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Try to build and run the mesh.
	errors := builder.Run()
	require.NotZero(suite.T(), len(errors), "builder was able run with processor to processor query relation")

	//Check that no instance were added to the failed to build mesh.
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 0, processors)
}

func (suite *BuilderTestSuite) TestBuilder__RunWithShutdown() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: Instance2
  destination: Instance1
  eventType: DummyEventType
queryRelations:
- source: Instance1
  destination: Instance2
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Instance1 is a Processor type
	err = builder.AddConstructor("Type1", processor.NewTestProcessor, &processor.TestProcessorParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Instance2 is a Service type
	err = builder.AddConstructor("Type2", processor.NewTestService, &processor.TestServiceParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Build and run the mesh.
	errors := builder.Run()
	require.Zero(suite.T(), len(errors), "builder run failed: %v", errors)

	//Check that both processors are ready.
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		err := wait.Poll(2*time.Millisecond, 2*time.Second, func() (bool, error) {
			current, err := iter.Current()
			if err != nil {
				return false, err
			}
			return current.instance.IsReady(), nil
		})
		require.NoError(suite.T(), err, "processor %d is not ready", processors)
		processors++
	}
	require.Equal(suite.T(), 2, processors)

	//Shutdown the mesh.
	errors = builder.Shutdown()
	require.Zero(suite.T(), len(errors), "builder shutdown failed: %v", errors)

	//Check that both processors are unready.
	processors = 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		err := wait.Poll(2*time.Millisecond, 2*time.Second, func() (bool, error) {
			current, err := iter.Current()
			if err != nil {
				return false, err
			}
			return !current.instance.IsReady(), nil
		})
		require.NoError(suite.T(), err, "processor %d is still ready", processors)
		processors++
	}
	require.Equal(suite.T(), 2, processors)

	//Test builder maps cleanup
	builder.Clear()
	processors = 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 0, processors)
	require.Equal(suite.T(), 0, len(builder.constructors))
}

func (suite *BuilderTestSuite) TestBuilder__BadRun() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Add constuctor which returns a processor with error on Run.
	err = builder.AddConstructor("Type1", newBadProcessor, &badProcessorParams{
		runError: true,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Try to build and run the mesh.
	errors := builder.Run()
	require.NotZero(suite.T(), len(errors), "builder was able to run with no errors")

	//Check that the processor reference was added to instances map as constructor worked.
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 1, processors)
}

func (suite *BuilderTestSuite) TestBuilder__BadShutdown() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Add constuctor which returns a processor with error on Shutdown.
	err = builder.AddConstructor("Type1", newBadProcessor, &badProcessorParams{
		shutdownError: true,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Try to build and run the mesh.
	errors := builder.Run()
	require.Zero(suite.T(), len(errors), "builder run failed: %v", errors)

	//Check that the processor reference was added to instances map.
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 1, processors)

	//Try a bad shutdown.
	errors = builder.Shutdown()
	require.NotZero(suite.T(), len(errors), "builder shutdown did not fail")
}

func (suite *BuilderTestSuite) TestBuilder__BadAddEventRelation() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: Instance1
  destination: Instance2
  eventType: DummyEventType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Instance1 is a processor which errors on adding event sink
	err = builder.AddConstructor("Type1", newBadProcessor, &badProcessorParams{
		addEventSinkError: true,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Instance2 is a processor type.
	err = builder.AddConstructor("Type2", processor.NewProcessorTap, &processor.TestProcessorParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Build the mesh. Should fail on adding event sink.
	errors := builder.Run()
	require.NotZero(suite.T(), len(errors), "builder run did not fail")

	//Check that no instance were added to the failed to build mesh.
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 0, processors)
}

func (suite *BuilderTestSuite) TestBuilder__BadAddEQueryRelation() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- source: Instance1
  destination: Instance2
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Instance1 is a processor which errors on adding query to service sink
	err = builder.AddConstructor("Type1", newBadProcessor, &badProcessorParams{
		addEventSinkError: true,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Instance2 is a service type.
	err = builder.AddConstructor("Type2", processor.NewServiceTap, &processor.TestServiceParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Build the mesh. Should fail on adding query sink.
	errors := builder.Run()
	require.NotZero(suite.T(), len(errors), "builder run did not fail")

	//Check that no instance were added to the failed to build mesh.
	processors := 0
	for iter := builder.GetProcessorsIterator(); iter != nil; iter = iter.Next() {
		processors++
	}
	require.Equal(suite.T(), 0, processors)
}

func (suite *BuilderTestSuite) TestBuilder__DoubleRunCall() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: Instance2
  destination: Instance1
  eventType: DummyEventType
queryRelations:
- source: Instance1
  destination: Instance2
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	builder, err := NewBuilder(file.Name())
	require.NoError(suite.T(), err, "failed to create builder: %s", err)

	//Instance1 is a Processor type
	err = builder.AddConstructor("Type1", processor.NewTestProcessor, &processor.TestProcessorParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Instance2 is a Service type
	err = builder.AddConstructor("Type2", processor.NewTestService, &processor.TestServiceParams{
		LivenessInterval: time.Second,
	})
	require.NoError(suite.T(), err, "failed to add constructor: %s", err)

	//Build and run the mesh.
	errors := builder.Run()
	require.Zero(suite.T(), len(errors), "builder run failed: %v", errors)

	//Duplicate Run call should fail.
	errors = builder.Run()
	require.NotZero(suite.T(), len(errors), "builder duplicate run call did not fail")
}

func TestBuilder__RUN(t *testing.T) {
	crt := new(BuilderTestSuite)
	suite.Run(t, crt)
}
