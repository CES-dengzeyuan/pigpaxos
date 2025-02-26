package hlc

import (
	"sync"
	"time"
)

var (
	//HLC
	HLClock *HLC
)

func CurrentTimeInMS() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

type HLC struct {
	lastWallTime int64
	currentHLC   *Timestamp

	sync.RWMutex
}

func init() {
	pt := CurrentTimeInMS()
	HLClock = NewHLC(pt)
}

//initialize HLC with a given physical time
func NewHLC(pt int64) *HLC {
	t := Timestamp{LogicalTime: 0, PhysicalTime: pt}
	hlc := HLC{currentHLC: &t, lastWallTime: pt}
	return &hlc
}

func (hlc *HLC) ReadClock() Timestamp {
	return *hlc.currentHLC //return timestamp
}

func (hlc *HLC) Now() Timestamp {
	hlc.Lock()
	defer hlc.Unlock()

	pt := CurrentTimeInMS()
	if hlc.currentHLC.GetPhysicalTime() >= pt {
		hlc.currentHLC.IncrementLogical()
	} else {
		hlc.currentHLC.SetPhysicalTime(pt)
		hlc.currentHLC.ResetLogical()
	}
	return *hlc.currentHLC //return timestamp

}

func (hlc *HLC) Update(t Timestamp) Timestamp {
	hlc.Lock()
	defer hlc.Unlock()

	pt := CurrentTimeInMS()

	if pt > hlc.currentHLC.GetPhysicalTime() && pt > t.GetPhysicalTime() {
		// Our physical clock is ahead of both wall times. It is used
		// as the new wall time and the logical clock is reset.
		hlc.currentHLC.SetPhysicalTime(pt)
		hlc.currentHLC.ResetLogical()
		return *hlc.currentHLC
	}

	if t.GetPhysicalTime() > hlc.currentHLC.GetPhysicalTime() {
		hlc.currentHLC.SetPhysicalTime(t.GetPhysicalTime())
		hlc.currentHLC.SetLogicalTime(t.GetLogicalTime() + 1)
	} else if hlc.currentHLC.GetPhysicalTime() > t.GetPhysicalTime() {
		hlc.currentHLC.IncrementLogical()
	} else {
		if t.GetLogicalTime() > hlc.currentHLC.GetLogicalTime() {
			hlc.currentHLC.SetLogicalTime(t.GetLogicalTime())
		}
		hlc.currentHLC.IncrementLogical()
	}
	return *hlc.currentHLC
}
