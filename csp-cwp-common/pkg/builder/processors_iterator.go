package builder

import (
	"fmt"

	omap "github.com/elliotchance/orderedmap"
)

//Iterator retruned by the Builder for iterating over its
//instance map entries in order of insertions.
type ProcessorsIterator struct {
	current *omap.Element
}

//Get iterator to Builder's Processors map entries so user could iterate over all
//existing processors (as for health checks).
func (b *Builder) GetProcessorsIterator() *ProcessorsIterator {
	if b.localInstances == nil || b.localInstances.Len() == 0 {
		return nil
	}
	return &ProcessorsIterator{
		current: b.localInstances.Front(),
	}
}

//Get next entry iterator or nil if finished all entries.
func (iter *ProcessorsIterator) Next() *ProcessorsIterator {
	if iter.current == nil || iter.current.Next() == nil {
		return nil
	}
	return &ProcessorsIterator{
		current: iter.current.Next(),
	}
}

//Get ProcessorInfo pointed currently by the iterator.
//Return nil if reached end of list. Error is returned in case of
//an unexpected ProcessorInfo entry within the entries list.
func (iter *ProcessorsIterator) Current() (*ProcessorInfo, error) {
	if iter.current == nil {
		return nil, nil
	}
	info, ok := (iter.current.Value).(*ProcessorInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected processor info entry in instances map")
	}
	return info, nil
}
