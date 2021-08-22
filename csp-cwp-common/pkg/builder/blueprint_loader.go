package builder

import (
	"fmt"
	"io/ioutil"

	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"

	yaml "gopkg.in/yaml.v2"
)

//TODO: support remote instances config
type blueprintLoader struct {
	localInstances []map[string]string
	eventRelations []map[string]string
	queryRelations []map[string]string
}

//Load the blueprint YAML file into the inner struct maps.
//File should have 3 main sections of map lists:
//
//# Listing local Processors and Services instances to create and run.
//# Instances will be created and run in order of their listing.
//localInstances:
// - name: <processor name>
//   type: <processor type>
//# Secifiying the event relations between instances
//eventRelations:
// - source: <processor name>
//   destination: <processor name>
//   eventType: <event type>
//# Secifiying the query relations between instances
//queryRelations:
// - source: <processor name>
//   destination: <processor name>
//   queryType: <query type>
//
//First section is considered mandatory, other two are optional
//but are most likely to appear as well.
func (b *blueprintLoader) load(filepath string) error {
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}

	var layout map[string][]map[string]string
	if err := yaml.Unmarshal(content, &layout); err != nil {
		return err
	}

	b.localInstances = layout["localInstances"]
	b.eventRelations = layout["eventRelations"]
	b.queryRelations = layout["queryRelations"]

	return b.validate()
}

//Validates the read strings map
func (b *blueprintLoader) validate() error {
	if len(b.localInstances) == 0 {
		return fmt.Errorf("missing localInstances information")
	}
	//Tracking of already met instance names
	instances := make(map[string]struct{})
	instanceExists := func(name string) bool {
		_, exists := instances[name]
		return exists
	}
	//Check local instances section
	for _, instanceInfo := range b.localInstances {
		//Check that each instance entry has name and type entries and their values
		//are not empty
		if err := b.checkKeys([]string{"name", "type"}, instanceInfo); err != nil {
			return err
		}
		name := instanceInfo["name"]
		//Check for duplicate instance declaration
		if instanceExists(name) {
			return fmt.Errorf("duplicate instance %s in instances map", name)
		} else {
			instances[name] = struct{}{}
		}
	}
	//Check event relations
	for _, eventRelation := range b.eventRelations {
		//Check that each eventRelation entry has source, destination and eventType
		//entries and their values are not empty.
		if err := b.checkKeys([]string{"source", "destination", "eventType"}, eventRelation); err != nil {
			return err
		}
		//Check that source refers to a defined instance name.
		source := eventRelation["source"]
		if !instanceExists(source) {
			return fmt.Errorf("unknown event source instance %s", source)
		}
		//Check that destination refers to a defined instance name.
		dest := eventRelation["destination"]
		if !instanceExists(dest) {
			return fmt.Errorf("unknown event destination instance %s", dest)
		}
		//Check that eventType refers to a proto defined event type.
		eventType := eventRelation["eventType"]
		if _, exists := proto.EventType_value[eventType]; !exists {
			return fmt.Errorf("invalid event type %s", eventType)
		}
	}
	//Check query relations
	for _, queryRelation := range b.queryRelations {
		//Check that each queryRelation entry has source, destination and queryType
		//entries and their values are not empty.
		if err := b.checkKeys([]string{"source", "destination", "queryType"}, queryRelation); err != nil {
			return err
		}
		//Check that source refers to a defined instance name.
		source := queryRelation["source"]
		if !instanceExists(source) {
			return fmt.Errorf("unknown query source instance %s", source)
		}
		//Check that destination refers to a defined instance name.
		dest := queryRelation["destination"]
		if !instanceExists(dest) {
			return fmt.Errorf("unknown query destination instance %s", dest)
		}
		//Check that queryType refers to a proto defined query type.
		queryType := queryRelation["queryType"]
		if _, exists := proto.QueryType_value[queryType]; !exists {
			return fmt.Errorf("invalid query type %s", queryType)
		}
	}
	return nil
}

//Check that givne listed key exist on string mape and that their values are not empty.
func (b *blueprintLoader) checkKeys(keys []string, info map[string]string) error {
	for _, key := range keys {
		if _, exists := info[key]; !exists {
			return fmt.Errorf("key %s is missing from map", key)
		}
		if info[key] == "" {
			return fmt.Errorf("empty value for key %s", key)
		}
	}
	return nil
}

//BlueprintLoader constructor.
func newBlueprintLoader(filepath string) (*blueprintLoader, error) {
	loader := &blueprintLoader{}
	if err := loader.load(filepath); err != nil {
		return nil, err
	}
	return loader, nil
}
