package builder

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BlueprintLoaderTestSuite struct {
	suite.Suite
}

func (suite *BlueprintLoaderTestSuite) SetupTest() {
}

func (suite *BlueprintLoaderTestSuite) TearDownTest() {
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingFile() {
	_, err := newBlueprintLoader("/no/such/file")
	require.Error(suite.T(), err, "no error for a missing blueprint file")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__EmptyFile() {
	file, err := createTemporaryFile([]byte{})
	require.NoError(suite.T(), err, "failed to create empty layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint without instances section")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__LoadLayout() {
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
queryRelations:
- source: Instance1
  destination: Instance2
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	loader, err := newBlueprintLoader(file.Name())
	require.NoError(suite.T(), err, "failed to load blueprint: %s", err)

	expectedInstances := []map[string]string{
		{
			"name": "Instance1",
			"type": "Type1",
		},
		{
			"name": "Instance2",
			"type": "Type2",
		},
	}
	require.True(suite.T(), reflect.DeepEqual(loader.localInstances, expectedInstances), "result=%v expected=%v", loader.localInstances, expectedInstances)

	expectedEventRelations := []map[string]string{
		{
			"source":      "Instance1",
			"destination": "Instance2",
			"eventType":   "DummyEventType",
		},
	}
	require.True(suite.T(), reflect.DeepEqual(loader.eventRelations, expectedEventRelations), "result=%v expected=%v", loader.localInstances, expectedEventRelations)

	expectedQueryRelations := []map[string]string{
		{
			"source":      "Instance1",
			"destination": "Instance2",
			"queryType":   "DummyQueryType",
		},
	}
	require.True(suite.T(), reflect.DeepEqual(loader.queryRelations, expectedQueryRelations), "result=%v expected=%v", loader.localInstances, expectedQueryRelations)
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__UnknownEventSourceInstance() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: UFO
  destination: Instance2
  eventType: DummyEventType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with unknown event source instance")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__DuplicateInstanceName() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance1
  type: Type2
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with duplicated instance name")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingValue() {
	layout := `
localInstances:
- name: Instance1
  type:
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing key value")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__BrokenLayout() {
	layout := `
localInstances:
- name: Instance1
  type
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with broken layout file")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__UnknownEventDestinationInstance() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: Instance1
  destination: UFO
  eventType: DummyEventType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with unknown event destination instance")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__UnknownEventType() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: Instance1
  destination: Instance2
  eventType: UFO
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with unknown event type")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__UnknownQuerySourceInstance() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- source: UFO
  destination: Instance2
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with unknown query source instance")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__UnknownQueryDestinationInstance() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- source: Instance1
  destination: UFO
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with unknown query destination instance")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__UnknownQueryType() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- source: Instance1
  destination: Instance2
  queryType: UFO
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with unknown query type")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingInstanceName() {
	layout := `
localInstances:
- type: Type1
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing instance name")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingInstanceType() {
	layout := `
localInstances:
- name: Instance1
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing instance type")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingEventSource() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- destination: Instance2
  eventType: DummyEventType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing event source")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingEventDestination() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: Instance1
  eventType: DummyEventType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing event destination")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingEventType() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
eventRelations:
- source: Instance1
  destination: Instance2
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing event type")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingQuerySource() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- destination: Instance2
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing query source")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingQueryDestination() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- source: Instance1
  queryType: DummyQueryType
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing query destination")
}

func (suite *BlueprintLoaderTestSuite) TestBlueprintLoader__MissingQueryType() {
	layout := `
localInstances:
- name: Instance1
  type: Type1
- name: Instance2
  type: Type2
queryRelations:
- source: Instance1
  destination: Instance2
`
	file, err := createTemporaryFile([]byte(layout))
	require.NoError(suite.T(), err, "failed to create layout file: %s", err)
	defer os.Remove(file.Name())

	_, err = newBlueprintLoader(file.Name())
	require.Error(suite.T(), err, "loaded blueprint with missing query type")
}

//Helper function for creating a temporary file with specific contents
func createTemporaryFile(content []byte) (*os.File, error) {
	file, err := ioutil.TempFile("", "blueprint_")
	if err != nil {
		return nil, err
	}

	if _, err = file.Write(content); err != nil {
		return nil, err
	}

	return file, nil
}

func TestBlueprintLoader__RUN(t *testing.T) {
	crt := new(BlueprintLoaderTestSuite)
	suite.Run(t, crt)
}
