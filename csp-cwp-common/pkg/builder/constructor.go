package builder

import (
	"fmt"
	"reflect"

	"github.com/rapid7/csp-cwp-common/pkg/processor"
)

type Creator interface{}
type Param interface{}

//The inner constructor entity of the builder:
//used to store and call the processor creating function with optional parameters.
//the call() method of this struct is used to validate and create the Processor by the
//builder when creating the processors mesh.
type constructor struct {
	creator Creator
	params  []Param
}

//The constructor invocation call:
//Return new Processor if all went well.
//Return nil Processor and an error if creator failed on validation or the actual call failed.
func (c *constructor) call() (processor.ProcessorInterface, error) {
	//Validate the provided creator function and parameters
	if err := c.validate(); err != nil {
		return nil, err
	}

	//Call the constructor function with the provided parameters
	paramValues := []reflect.Value{}
	for _, p := range c.params {
		paramValues = append(paramValues, reflect.ValueOf(p))
	}
	constructorValue := reflect.ValueOf(c.creator)
	resultValues := constructorValue.Call(paramValues)

	//Convert the returned error
	if len(resultValues) > 1 {
		if anyObj := resultValues[1].Interface(); anyObj != nil {
			err, ok := anyObj.(error)
			if !ok {
				return nil, fmt.Errorf("failed to convert returned error")
			}
			if err != nil {
				return nil, err
			}
		}
	}

	//Convert the returned Processor
	if anyObj := resultValues[0].Interface(); anyObj != nil {
		proc, ok := anyObj.(processor.ProcessorInterface)
		if !ok {
			return nil, fmt.Errorf("failed to convert returned ProcessorInterface")
		}
		return proc, nil
	}
	return nil, fmt.Errorf("constructor returned nil processor")
}

//Validate the constructor, return error if something went wrong.
//Accepted creator functions prototypes have the following forms
//and are expected to be provided with the matching parameters list
//both in number and in types:
//
//func NewProcessor(param1 Type1,... , paramN TypeN) (ProcessorInterface, error)
//func NewService(param1 Type1,... , paramN TypeN) (ServiceInterface, error)
//func NewProcessor() (ProcessorInterface, error)
//func NewService() (ServiceInterface, error)
//func NewProcessor(param1 Type1,... , paramN TypeN) ProcessorInterface
//func NewService(param1 Type1,... , paramN TypeN) ServiceInterface
//func NewProcessor() ProcessorInterface
//func NewService() ServiceInterface
func (c *constructor) validate() error {
	if c.creator == nil {
		return fmt.Errorf("missing creator function")
	}

	creatorType := reflect.TypeOf(c.creator)
	//Creator function should be provided with number of param values as declared on its prototype.
	//Example: func NewProcessor(param1 int) (ProcessorInterface, error)
	//Rejected when provided with: param1 int, param2 string
	if numIn := creatorType.NumIn(); numIn != len(c.params) {
		return fmt.Errorf("unexpected number of parameters (expects: %d got: %d)", numIn, len(c.params))
	}

	//Creator function should be provided with matching parameters types
	//Example: func NewProcessor(param1 int) (ProcessorInterface, error)
	//Rejected when provided with: param1 string
	for i, p := range c.params {
		expects := creatorType.In(i).String()
		got := reflect.TypeOf(p).String()
		if got != expects {
			return fmt.Errorf("mismatching type for param #%d (expects: %s got: %s)", i+1, expects, got)
		}
	}
	//Creator function should return at least a processor interface and optional error
	//or an extension of those interfaces.
	//Example1: func NewProcessor() (ProcessorInterface, error)
	//Example2: func NewProcessor() ProcessorInterface
	//Example3: func NewService() (ServiceInterface, error)
	//Example4: func NewService() ServiceInterface
	numOut := creatorType.NumOut()
	if numOut < 1 || numOut > 2 {
		return fmt.Errorf("unexpected number of returned values %d", numOut)
	}
	processorInterfaceType := reflect.TypeOf((*processor.ProcessorInterface)(nil)).Elem()
	if !creatorType.Out(0).Implements(processorInterfaceType) {
		return fmt.Errorf("first return type does not implement Processor interface")
	}
	errorInterfaceType := reflect.TypeOf((*error)(nil)).Elem()
	if numOut > 1 && !creatorType.Out(1).Implements(errorInterfaceType) {
		return fmt.Errorf("second return type does not implement error interface")
	}

	return nil
}

//Create new constructor struct:
//creator is the creator function pointer to be validated and called.
//params is an optional list of params to be passed to the creator function.
//Note: Using variadic args for params here so user will not be forced to specify
//empty param list in case there are none to pass.
func newConstructor(creator Creator, params ...Param) *constructor {
	return &constructor{
		creator: creator,
		params:  params,
	}
}
