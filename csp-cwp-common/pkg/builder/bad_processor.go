//NOTE: the file is only for unittesting purpose

package builder

import (
	"fmt"
	"time"

	"github.com/rapid7/csp-cwp-common/pkg/processor"
	proto "github.com/rapid7/csp-cwp-common/pkg/proto/processor"
)

type badProcessorParams struct {
	shutdownError     bool
	runError          bool
	addEventSinkError bool
	addQuerySinkError bool
}

type badProcessor struct {
	processor.ProcessorInterface
	shutdownError     bool
	runError          bool
	addEventSinkError bool
	addQuerySinkError bool
}

func (bp *badProcessor) Run() error {
	if bp.runError {
		return fmt.Errorf("run error")
	}
	return nil
}

func (bp *badProcessor) Shutdown() error {
	if bp.shutdownError {
		return fmt.Errorf("shutdown error")
	}
	return nil
}

func (bp *badProcessor) AddEventSink(eventType proto.EventType, sink processor.SinkInterface) error {
	if bp.addEventSinkError {
		return fmt.Errorf("failed to add event sink")
	}
	return nil
}

func (bp *badProcessor) AddEventQuery(queryType proto.QueryType, sink processor.SinkInterface) error {
	if bp.addQuerySinkError {
		return fmt.Errorf("failed to add query sink")
	}
	return nil
}

func (bp *badProcessor) PushEvent(event *proto.Event) error {
	return nil
}

func (bp *badProcessor) UpdateConfiguration(conf *proto.Configuration) error {
	return nil
}

func (bp *badProcessor) IsAlive(gracePeriod time.Duration) bool {
	return false
}

func (bp *badProcessor) IsReady() bool {
	return false
}

func (bp *badProcessor) GetHeartbeat() proto.Heartbeat {
	return proto.Heartbeat{}
}

func newBadProcessor(param *badProcessorParams) processor.ProcessorInterface {
	return &badProcessor{
		runError:          param.runError,
		shutdownError:     param.shutdownError,
		addEventSinkError: param.addEventSinkError,
		addQuerySinkError: param.addQuerySinkError,
	}
}
