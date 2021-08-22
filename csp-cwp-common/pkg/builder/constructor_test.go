package builder

import (
	"fmt"
	"testing"
	"time"

	"github.com/rapid7/csp-cwp-common/pkg/processor"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ConstructorTestSuite struct {
	suite.Suite
}

func (suite *ConstructorTestSuite) SetupTest() {
}

func (suite *ConstructorTestSuite) TearDownTest() {
}

func (suite *ConstructorTestSuite) TestConstructor__MissingParam() {
	//NewTestProcessor(...) expects a single parameter
	ctor := newConstructor(processor.NewTestProcessor)
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructed processor with a missing creator param")
}

func (suite *ConstructorTestSuite) TestConstructor__TooManyParams() {
	//NewTestProcessor(...) expects a single parameter
	param1 := &processor.TestProcessorParams{}
	ctor := newConstructor(processor.NewTestProcessor, param1, "extra-param")
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructed processor with mismatching params list")
}

func (suite *ConstructorTestSuite) TestConstructor__MismatchingParamType() {
	ctor := newConstructor(processor.NewTestProcessor, "not-your-param-type")
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructed processor with mismatching param type")
}

func (suite *ConstructorTestSuite) TestConstructor__MissingReturnValue() {
	emptyFunc := func() {}
	ctor := newConstructor(emptyFunc)
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructor with a missing return value")
}

func (suite *ConstructorTestSuite) TestConstructor__InvalidProcessorReturnType() {
	invalidProcessorConstructor := func() int { return 42 }
	ctor := newConstructor(invalidProcessorConstructor)
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructor does not return ProcessorInterface")
}

func (suite *ConstructorTestSuite) TestConstructor__InvalidErrorReturnType() {
	invalidErrorType := func() (processor.ProcessorInterface, int) {
		return processor.NewTestProcessor(&processor.TestProcessorParams{}), 42
	}
	ctor := newConstructor(invalidErrorType)
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructor does not return a valid error type")
}

func (suite *ConstructorTestSuite) TestConstructor__TooManyReturnValues() {
	invalidErrorType := func() (processor.ProcessorInterface, error, error) {
		return processor.NewTestProcessor(&processor.TestProcessorParams{}), nil, nil
	}
	ctor := newConstructor(invalidErrorType)
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructor returns too many values")
}

func (suite *ConstructorTestSuite) TestConstructor__CreateProcessorWithNoError() {
	processorConstructor := func(params *processor.TestProcessorParams) (processor.ProcessorInterface, error) {
		return processor.NewTestProcessor(params), nil
	}
	param := &processor.TestProcessorParams{}
	ctor := newConstructor(processorConstructor, param)
	_, err := ctor.call()
	require.NoError(suite.T(), err, "constructor failed: %s", err)
}

func (suite *ConstructorTestSuite) TestConstructor__CreateProcessorWithError() {
	processorConstructor := func(params *processor.TestProcessorParams) (processor.ProcessorInterface, error) {
		return processor.NewTestProcessor(params), fmt.Errorf("some test error")
	}
	param := &processor.TestProcessorParams{}
	ctor := newConstructor(processorConstructor, param)
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructor should fail due to callee error")
}

func (suite *ConstructorTestSuite) TestConstructor__CreateNilProcessor() {
	processorConstructor := func() (processor.ProcessorInterface, error) {
		return nil, nil
	}
	ctor := newConstructor(processorConstructor)
	_, err := ctor.call()
	require.Error(suite.T(), err, "constructor should fail due to returning nil processor")
}

func (suite *ConstructorTestSuite) TestConstructor__ConstructProcessor() {
	//Create constructor entity
	param := &processor.TestProcessorParams{
		LivenessInterval: time.Second,
	}
	ctor := newConstructor(processor.NewTestProcessor, param)

	//Create the processor from the constructor entity
	proc, err := ctor.call()
	require.NoError(suite.T(), err, "failed to construct processor: %s", err)

	//Check created processor for no readiness
	require.False(suite.T(), proc.IsReady(), "processor marked as ready before run")

	//Run created processor and check for readiness
	err = proc.Run()
	require.NoError(suite.T(), err, "failed to run processor: %s", err)
	err = wait.Poll(10*time.Millisecond, 2*time.Second, func() (bool, error) { return proc.IsReady(), nil })
	require.NoError(suite.T(), err, "running processor is not ready: %s", err)

	//Shutdown created processor and wait for no readiness
	err = proc.Shutdown()
	require.NoError(suite.T(), err, "failed to shutdown processor: %s", err)
	err = wait.Poll(10*time.Millisecond, 2*time.Second, func() (bool, error) { return !proc.IsReady(), nil })
	require.NoError(suite.T(), err, "shut processor remained ready: %s", err)
}

func (suite *ConstructorTestSuite) TestConstructor__ConstructService() {
	//Create constructor entity
	param := &processor.TestServiceParams{
		LivenessInterval: time.Second,
	}
	ctor := newConstructor(processor.NewTestService, param)

	//Create the service from the constructor entity
	service, err := ctor.call()
	require.NoError(suite.T(), err, "failed to construct service: %s", err)

	//Check created service for no readiness
	require.False(suite.T(), service.IsReady(), "service marked as ready before run")

	//Run created service and check for readiness
	err = service.Run()
	require.NoError(suite.T(), err, "failed to run service: %s", err)
	err = wait.Poll(10*time.Millisecond, 2*time.Second, func() (bool, error) { return service.IsReady(), nil })
	require.NoError(suite.T(), err, "running service is not ready: %s", err)

	//Shutdown created service and wait for no readiness
	err = service.Shutdown()
	require.NoError(suite.T(), err, "failed to shutdown service: %s", err)
	err = wait.Poll(10*time.Millisecond, 2*time.Second, func() (bool, error) { return !service.IsReady(), nil })
	require.NoError(suite.T(), err, "shut service remained ready: %s", err)
}

func TestConstructor__RUN(t *testing.T) {
	crt := new(ConstructorTestSuite)
	suite.Run(t, crt)
}
