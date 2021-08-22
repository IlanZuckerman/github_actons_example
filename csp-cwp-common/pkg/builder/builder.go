package builder

import (
	"fmt"

	"github.com/rapid7/csp-cwp-common/pkg/processor"
	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"

	omap "github.com/elliotchance/orderedmap"
)

type ProcessorInfo struct {
	//ServiceInterface extends ProcessorInterface so both can be mapped to base type.
	instance processor.ProcessorInterface
	//TODO: keep additional remote information here as well.
}

//Definition of the main processors builder:
//This entity should know how to instantiate all the system Processors and Services
//along with their Event and Query relations given a blueprint mapping.
type Builder struct {
	//The mesh blueprint loader
	loader *blueprintLoader
	//Mapping from an instance type to its constructor.
	constructors map[string]*constructor
	//Track instances information in order of creation in case the startup order is important.
	localInstances *omap.OrderedMap
	//TODO: support remote instances
}

//Add processor constructor to builder's constructors map:
//typeName is the processor implementation type identifier.
//creator is the function used to create the specific Processor or Service of this type.
//params is an optional list of params to be passed to the creator function.
//Note: Using variadic args for params here so user will not be forced to specify
//empty param list in case there are none to pass.
func (b *Builder) AddConstructor(typeName string, creator Creator, params ...Param) error {
	if _, exists := b.constructors[typeName]; exists {
		return fmt.Errorf("constructor %s already exists", typeName)
	}
	b.constructors[typeName] = newConstructor(creator, params...) //variadic passthrough
	return nil
}

//Clear the mesh and constructors map.
func (b *Builder) Clear() {
	b.clearMesh()
	b.constructors = make(map[string]*constructor)
}

//Create and run the processors in same order as they were listed on blueprint.
//Return list of encountered errors.
//NOTE: Shutdown API is not called automatically in case of a Run error as it would be cumbersome
//to track back also possible Shutdown erros added to same Run errors list.
//The user of the Builder API should check if Run call had errors and call the Shutdown API
//to shut off any running processors which were able to run.
func (b *Builder) Run() []error {
	//Check for prvious mesh
	if b.localInstances.Len() > 0 {
		return []error{
			fmt.Errorf("mesh was already run"),
		}
	}

	//Create the processors mesh and cleanup on error
	if err := b.createProcessorsMesh(); err != nil {
		b.clearMesh()
		return []error{
			err,
		}
	}

	//Run the processors
	errors := []error{}
	for entry := b.localInstances.Front(); entry != nil; entry = entry.Next() {
		if info, ok := (entry.Value).(*ProcessorInfo); !ok {
			errors = append(errors, fmt.Errorf("unexpected processor info entry in instances map"))
		} else if err := info.instance.Run(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

//Shutdown the processors in their reverse startup order.
//Return list of encountered errors.
func (b *Builder) Shutdown() []error {
	errors := []error{}
	for entry := b.localInstances.Back(); entry != nil; entry = entry.Prev() {
		if info, ok := (entry.Value).(*ProcessorInfo); !ok {
			errors = append(errors, fmt.Errorf("unexpected processor info entry in instances map"))
		} else if err := info.instance.Shutdown(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

//Create the processors mesh from the read blueprint.
//TODO: support remote processors.
func (b *Builder) createProcessorsMesh() error {
	//Create the local Processor instances
	for _, info := range b.loader.localInstances {
		if err := b.createProcessor(info["type"], info["name"]); err != nil {
			return err
		}
	}

	//Create event relations
	for _, relation := range b.loader.eventRelations {
		eventType := proto.EventType_value[relation["eventType"]]
		if err := b.addEventRelation(relation["source"], relation["destination"], proto.EventType(eventType)); err != nil {
			return err
		}
	}

	//Create query relations
	for _, relation := range b.loader.queryRelations {
		queryType := proto.QueryType_value[relation["queryType"]]
		if err := b.addQueryRelation(relation["source"], relation["destination"], proto.QueryType(queryType)); err != nil {
			return err
		}
	}
	return nil
}

//Create a processor and add it into ordered instances map
func (b *Builder) createProcessor(typeName string, name string) error {
	if _, exists := b.localInstances.Get(name); exists {
		return fmt.Errorf("instance name %s already exists", name)
	}

	//Instantiate the processor according to its type
	ctor, exists := b.constructors[typeName]
	if !exists {
		return fmt.Errorf("failed to find constructor for instance type %s", typeName)
	}
	instance, err := ctor.call()
	if err != nil {
		return fmt.Errorf("creation of instance (%s, %s) failed: %s", typeName, name, err)
	}
	b.localInstances.Set(name, &ProcessorInfo{
		instance: instance,
	})
	return nil
}

//Clear the existing mesh
func (b *Builder) clearMesh() {
	for _, key := range b.localInstances.Keys() {
		b.localInstances.Delete(key)
	}
}

//Get entry from instances map
func (b *Builder) getProcessorInfo(name string) (*ProcessorInfo, error) {
	entry, exists := b.localInstances.Get(name)
	if !exists {
		return nil, fmt.Errorf("failed to find processor info for %s", name)
	}
	info, ok := entry.(*ProcessorInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected processor info entry for name %s", name)
	}
	return info, nil
}

//Add event relation:
//A sink for a tap of dest processor is added to event types map of source processor
func (b *Builder) addEventRelation(srcName string, dstName string, eventType proto.EventType) error {
	srcInfo, err := b.getProcessorInfo(srcName)
	if err != nil {
		return err
	}
	dstInfo, err := b.getProcessorInfo(dstName)
	if err != nil {
		return err
	}
	sink := processor.NewSink(dstInfo.instance.GetTap())
	err = srcInfo.instance.AddEventSink(eventType, sink)
	return err
}

//Add query relation:
//A sink for a tap of dest service is added to query types map of source processor
func (b *Builder) addQueryRelation(srcName string, dstName string, queryType proto.QueryType) error {
	srcInfo, err := b.getProcessorInfo(srcName)
	if err != nil {
		return err
	}
	dstInfo, err := b.getProcessorInfo(dstName)
	if err != nil {
		return err
	}
	if _, ok := (dstInfo.instance).(processor.ServiceInterface); !ok {
		return fmt.Errorf("destination must implement ServiceInterface in order to serve queries")
	}
	sink := processor.NewSink(dstInfo.instance.GetTap())
	err = srcInfo.instance.AddQuerySink(queryType, sink)
	return err
}

//The builder constructor gets a yaml file as a blueprint.
//TODO: support remote instances information.
func NewBuilder(blueprintFile string) (*Builder, error) {
	loader, err := newBlueprintLoader(blueprintFile)
	if err != nil {
		return nil, err
	}
	return &Builder{
		loader:         loader,
		constructors:   make(map[string]*constructor),
		localInstances: omap.NewOrderedMap(),
	}, nil
}
