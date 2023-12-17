package paxi

import (
	"pigpaxos/log"
	"sort"
	"strconv"
	"strings"
)

// ID represents a generic identifier in format of Zone.Node
type ID uint32

//type ID string

// NewID returns a new ID type given two int number of zone and node
func NewID(zone, node int) ID {
	return ID(zone<<16 | node)
}

func NewIDFromString(idstr string) ID {
	idParts := strings.Split(idstr, ".")
	if len(idParts) != 2 {
		log.Errorf("Invalid id: %v", idstr)
		return 0
	}
	zone, err := strconv.Atoi(idParts[0])
	if err != nil {
		log.Errorf("Invalid id: %v", idstr)
		return 0
	}

	node, err := strconv.Atoi(idParts[1])
	if err != nil {
		log.Errorf("Invalid id: %v", idstr)
		return 0
	}

	return NewID(zone, node)
}

// Zone returns Zone ID component
func (i ID) Zone() int {
	return int(i >> 16)
}

// Node returns Node ID component
func (i ID) Node() int {
	var z uint32 = 0x0000FFFF
	return int(z & (uint32(i)))
}

func (i ID) String() string {
	return strconv.Itoa(i.Zone()) + "." + strconv.Itoa(i.Node())
}

type IDs []ID

func (ids IDs) Len() int      { return len(ids) }
func (ids IDs) Swap(i, j int) { ids[i], ids[j] = ids[j], ids[i] }
func (ids IDs) Less(i, j int) bool {
	if ids[i].Zone() < ids[j].Zone() {
		return true
	} else if ids[i].Zone() > ids[j].Zone() {
		return false
	} else {
		return ids[i].Node() < ids[j].Node()
	}
}

func compareID(a, b ID) int {
	switch {
	case a.Zone() < b.Zone():
		return -1
	case a.Zone() > b.Zone():
		return 1
	}
	switch {
	case a.Node() < b.Node():
		return -1
	case a.Node() > b.Node():
		return 1
	}
	return 0
}

func (ids IDs) Sort() {
	// 使用 sort.Slice 函数对 ids 进行排序
	sort.Slice(ids, func(i, j int) bool {
		// 使用 compareID 函数比较 ids[i] 和 ids[j]
		return compareID(ids[i], ids[j]) < 0
	})
}
