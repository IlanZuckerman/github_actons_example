package processor

import (
	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

//Definition of Service interface
//This is the basic definition for any internal component processing
//and sending events within the agent.
type ServiceInterface interface {
	ProcessorInterface

	//Handle recieved query
	RunQuery(query *proto.Query) (*proto.QueryResult, error)
}
