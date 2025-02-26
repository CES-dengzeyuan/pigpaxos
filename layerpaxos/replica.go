package layerpaxos

import (
	"flag"
	"fmt"
	"math/rand"
	paxi "pigpaxos"
	"pigpaxos/log"
	"sort"
	"sync"
	"time"
)

const GrayTimeoutMultiplier = 1000
const TickerDuration = 10

var stableLeader = flag.Bool("sld", false, "stable leader, if true paxos forward request to current leader")
var pg = flag.Int("npg", 2, "Number of peer-groups. Default is 2")
var regionPeerGroups = flag.Bool("wrpg", false, "use region as a peer group instead")
var useSmallP2b = flag.Bool("usp2b", true, "use small p2b aggregated message that put missing IDs instead of voted ids")
var stdPigTimeout = flag.Int("ptt", 50, "Standard timeout after which all non-collected responses are treated as failures")
var rgSlack = flag.Int("nrgslack", 0, "Slack for Relay group waiting. Ignoring this many slowest nodes")
var fixedrelay = flag.Bool("wfr", false, "Use static relay nodes that do not randomly change")

type BalSlot struct {
	paxi.Ballot
	slot int
}

type RelayToLeader struct {
	relayTimeInt int64
	BalSlot
}

type PeerGroup struct {
	nodes []paxi.ID
}

func (pg *PeerGroup) GetRandomNodeId(excludeId paxi.ID, gray map[paxi.ID]time.Time) paxi.ID {
	randId := pg.nodes[rand.Intn(len(pg.nodes))]
	_, isgray := gray[randId]
	for randId == excludeId || isgray {
		randId = pg.nodes[rand.Intn(len(pg.nodes))]
		_, isgray = gray[randId]
	}
	return randId
}

func (pg PeerGroup) String() string {
	return fmt.Sprintf("PeerGroup {nodes=%v}", pg.nodes)
}

// Replica for one PigPaxos instance
type Replica struct {
	paxi.Node
	*LayerPaxos
	relayGroups       []*PeerGroup
	fixedRelays       []paxi.ID
	myRelayGroup      int
	NodeIdsToGroup    map[paxi.ID]int
	numRelayGroups    int
	maxDepth          uint8
	relaySlack        int
	cleanupMultiplier int

	GrayNodes map[paxi.ID]time.Time

	p1bRelayRoutedMsg *RoutedMsg
	pendingP1bRelay   int64
	p1bRelayDepth     uint8

	p2bRelaysMapByBalSlot     map[int]*RoutedMsg
	p2bRelaysTimeMapByBalSlot map[int]int64

	sync.RWMutex
	GrayLock sync.RWMutex
}

// NewReplica generates new Paxos replica
func NewReplica(id paxi.ID) *Replica {
	log.Debugf("LayerPaxos Starting replica %v", id)
	r := new(Replica)
	r.Node = paxi.NewNode(id)
	r.LayerPaxos = NewLayerPaxos(r)
	r.Register(paxi.Request{}, r.handleRequest)
	r.Register(P1b{}, r.handleP1b)
	r.Register([]P1b{}, r.handleP1bLeader)
	r.Register(P2b{}, r.handleP2b)
	r.Register(P2bAggregated{}, r.handleP2bAggregated)
	r.Register(P3{}, r.handleP3)
	r.Register(P3RecoverRequest{}, r.HandleP3RecoverRequest)
	r.Register(P3RecoverReply{}, r.HandleP3RecoverReply)
	r.Register(RoutedMsg{}, r.handleRoutedMsg)

	r.pendingP1bRelay = 0
	r.p1bRelayDepth = 0
	r.relaySlack = *rgSlack
	r.cleanupMultiplier = 3
	r.p2bRelaysMapByBalSlot = make(map[int]*RoutedMsg)
	r.p2bRelaysTimeMapByBalSlot = make(map[int]int64)
	r.NodeIdsToGroup = make(map[paxi.ID]int)
	r.GrayNodes = make(map[paxi.ID]time.Time)

	knownIDs := make([]paxi.ID, 0, len(paxi.GetConfig().Addrs))
	for id := range paxi.GetConfig().Addrs {
		knownIDs = append(knownIDs, id)
	}

	sort.Slice(knownIDs, func(i, j int) bool {
		return knownIDs[i].Zone() < knownIDs[j].Zone() ||
			(knownIDs[i].Zone() == knownIDs[j].Zone() && knownIDs[i].Node() < knownIDs[j].Node())
	})

	log.Debugf("Known IDs : %v", knownIDs)

	r.maxDepth = 2
	if !*regionPeerGroups {
		r.numRelayGroups = *pg
		r.relayGroups = r.peersToGroups(*pg, knownIDs)
		log.Infof("LayerPaxos computed PeerGroups: {%v}", r.relayGroups)
	} else {
		r.numRelayGroups = paxi.GetConfig().Z()
		r.relayGroups = make([]*PeerGroup, r.numRelayGroups)
		r.myRelayGroup = r.ID().Zone() - 1
		for _, id := range knownIDs {
			pgNum := id.Zone() - 1
			if r.relayGroups[pgNum] == nil {
				r.relayGroups[pgNum] = &PeerGroup{nodes: make([]paxi.ID, 0)}
			}
			r.relayGroups[pgNum].nodes = append(r.relayGroups[pgNum].nodes, id)
		}

		log.Infof("LayerPaxos region computed PeerGroups: {%v}", r.relayGroups)
	}

	r.fixedRelays = make([]paxi.ID, r.numRelayGroups)

	for i, pg := range r.relayGroups {
		for _, id := range pg.nodes {
			r.NodeIdsToGroup[id] = i
		}
		if *fixedrelay {
			r.fixedRelays[i] = r.relayGroups[i].GetRandomNodeId(r.ID(), r.GrayNodes)
		}
	}

	log.Infof("LayerPaxos region NodeIdsToPeerGroups: {%v}", r.NodeIdsToGroup)

	go r.startTicker()

	return r
}

func (r *Replica) peersToGroups(numGroups int, nodeList []paxi.ID) []*PeerGroup {
	peerGroups := make([]*PeerGroup, numGroups)
	pgNum := 0
	nodesAddToPg := 0
	nodesPerGroup := len(nodeList) / numGroups
	for _, id := range nodeList {
		if id == r.ID() {
			r.myRelayGroup = pgNum
		}
		if peerGroups[pgNum] == nil {
			peerGroups[pgNum] = &PeerGroup{nodes: make([]paxi.ID, 0)}
		}

		peerGroups[pgNum].nodes = append(peerGroups[pgNum].nodes, id)
		nodesAddToPg++
		if nodesAddToPg >= nodesPerGroup && pgNum+1 < numGroups {
			pgNum++
			nodesAddToPg = 0
		}
	}
	return peerGroups
}

//*********************************************************************************************************************
// Timer for all timed events, such as timeouts and log clean ups
//*********************************************************************************************************************
func (r *Replica) startTicker() {
	var ticks uint64 = 0
	for now := range time.Tick(TickerDuration * time.Millisecond) {
		// log cleanup
		ticks++
		if ticks%uint64(r.cleanupMultiplier) == 0 {
			r.CleanupLog()
		}

		if ticks%uint64(GrayTimeoutMultiplier) == 0 {
			log.Debugf("Ticker gray check on tick %d", ticks)
			r.GrayLock.Lock()
			for grayId, t := range r.GrayNodes {
				if t.Add(time.Duration(TickerDuration*GrayTimeoutMultiplier) * time.Millisecond).Before(now) {
					log.Infof("Removing node %v from gray list on timeout", grayId)
					delete(r.GrayNodes, grayId)
				}
			}
			r.GrayLock.Unlock()
			log.Debugf("Ticker gray check done on tick %d", ticks)
		}

		// handling timeouts
		timeoutCutoffTime := now.Add(-time.Duration(*stdPigTimeout) * time.Millisecond).UnixNano() // everything older than this needs to timeout
		//log.Debugf("Start TimeoutChecker (timeout_cutoff = %d)", timeoutCutoffTime)
		if r.IsLeader() {
			r.CheckTimeout(timeoutCutoffTime)
		} else {
			// check for P1b timeouts
			r.Lock()
			if r.p1bRelayRoutedMsg != nil {
				p1bs := r.p1bRelayRoutedMsg.Payload.([]P1b)
				if r.pendingP1bRelay > 0 && r.pendingP1bRelay < timeoutCutoffTime && len(p1bs) > 0 {
					// we have timeout on P1b
					log.Debugf("Timeout on P1b. Relaying p1bs {%v}", r.p1bRelayRoutedMsg.Payload)

					if r.p1bRelayRoutedMsg.Progress == 0 {
						r.Send(r.p1bRelayRoutedMsg.GetLastProgressHop(), p1bs)
					} else {
						r.Send(r.p1bRelayRoutedMsg.GetLastProgressHop(), r.p1bRelayRoutedMsg)
					}
					r.p1bRelayRoutedMsg = nil
					r.pendingP1bRelay = 0
				}
			}
			// check for p2b timeouts
			for slot, routedP2b := range r.p2bRelaysMapByBalSlot {
				if r.p2bRelaysTimeMapByBalSlot[slot] < timeoutCutoffTime {
					routedP2b.IsForward = false
					log.Debugf("Timeout on P2b. Relaying p2bs {%v}", r.p2bRelaysMapByBalSlot[slot])
					if routedP2b.Progress == 0 {
						p2b := routedP2b.Payload.(P2b)
						if *useSmallP2b {
							p2bSmall := P2bAggregated{
								Ballot:           p2b.Ballot,
								Slot:             p2b.Slot,
								RelayLastExecute: r.execute - 1,
								RelayID:          r.ID(),
								MissingIDs:       r.computeMissingIDsForP2b(p2b),
							}
							r.Send(routedP2b.GetLastProgressHop(), p2bSmall)
						} else {
							r.Send(routedP2b.GetLastProgressHop(), routedP2b.Payload)
						}
					} else {
						r.Send(routedP2b.GetLastProgressHop(), routedP2b)
					}
					delete(r.p2bRelaysMapByBalSlot, slot)
					delete(r.p2bRelaysTimeMapByBalSlot, slot)
				}
			}
			r.Unlock()
		}
	}
}

//*********************************************************************************************************************
// Messaging
//********************************************************************************************************************

// Overrides Broadcast in node
func (r *Replica) Broadcast(m interface{}) {
	log.Debugf("LayerPaxos Broadcast Msg: {%v}", m)
	routedMsg := RoutedMsg{
		Hops:      make([]paxi.ID, 1),
		IsForward: true,
		Progress:  0,
		Payload:   m,
	}
	routedMsg.Hops[0] = r.ID()
	for i := 0; i < r.numRelayGroups; i++ {
		var relayId paxi.ID
		if *fixedrelay {
			relayId = r.fixedRelays[i]
		} else {
			r.GrayLock.RLock()
			relayId = r.relayGroups[i].GetRandomNodeId(r.ID(), r.GrayNodes)
			r.GrayLock.RUnlock()
			log.Debugf("Generated Random Relay for RG #%d {%v}: %v", i, r.relayGroups[i], relayId)
		}
		r.Send(relayId, routedMsg)
	}
}

// special broadcast for messages within the peer group
func (r *Replica) BroadcastToPeerGroup(pg *PeerGroup, originalSourceToExclude paxi.ID, m RoutedMsg) {
	log.Debugf("LayerPaxos Broadcast to PeerGroup %v: {%v}", pg, m)
	for _, id := range pg.nodes {
		r.GrayLock.RLock()
		_, gray := r.GrayNodes[id]
		r.GrayLock.RUnlock()
		if id != r.ID() && id != originalSourceToExclude && !gray {
			go r.Send(id, m)
		}
	}
}

func (r *Replica) Send(to paxi.ID, m interface{}) error {
	if to == r.ID() {
		log.Debugf("LayerPaxos Self Send loop on Msg: {%v}", m)
		r.HandleMsg(m) // loopback for self
	} else {
		err := r.Node.Send(to, m)
		if err != nil {
			// add node to gray list
			r.GrayLock.Lock()
			log.Infof("Adding node %v to gray list", to)
			r.GrayNodes[to] = time.Now()
			r.GrayLock.Unlock()
		}
	}

	return nil
}

//*********************************************************************************************************************
// Routing
//*********************************************************************************************************************

func (r *Replica) handleRoutedMsg(m RoutedMsg) {
	log.Debugf("Node %v handling RoutedMsg {%v}", r.ID(), m)
	if m.IsForward {
		// handle the payload ourselves
		needToPropagate := false
		switch msg := m.Payload.(type) {
		case P1a:
			needToPropagate = r.handleP1aRelay(msg, m)
			if needToPropagate {
				r.p1bRelayDepth = m.Progress
			}
		case P2a:
			needToPropagate = r.handleP2aRelay(msg, m)
		case P3:
			log.Debugf("Node %v handling msg {%v}", r.ID(), msg)
			needToPropagate = true
			r.HandleP3(msg)
		}

		// forward propagation if needed
		if m.Progress+1 < r.maxDepth && needToPropagate {
			// still not done going to the leaf nodes
			m.Progress += 1
			pgToBroadcast := r.relayGroups[r.myRelayGroup]
			m.Hops = append(m.Hops, r.ID())
			log.Debugf("Node %v forward propagating msg %v at depth %d and max depth %d", r.ID(), m, m.Progress, r.maxDepth)
			r.BroadcastToPeerGroup(pgToBroadcast, m.GetPreviousProgressHop(), m)
		}
	} else {
		// backward propagation
		// we have different logic for back-propagating P1bs and P2bs
		switch relayPayload := m.Payload.(type) {
		case P1b:
			r.handleP1bRelay(relayPayload)
		case P2b:
			r.handleP2bRelay(relayPayload)
		}
	}
}

//*********************************************************************************************************************
// Forward Propagation
//*********************************************************************************************************************

//func (r *Replica) handleP1a(m P1a) {
//	log.Debugf("Node %v handling msg {%v}", r.ID(), m)
//	r.HandleP1a(m, m.Ballot.ID())
//}

func (r *Replica) handleP1aRelay(m P1a, routedMsg RoutedMsg) bool {
	needToPropagate := false
	log.Debugf("Node %v handling p1aRelay msg {%v}", r.ID(), m)
	oldBallot := r.Ballot()
	if oldBallot < m.Ballot {
		if routedMsg.Progress+1 < r.maxDepth {
			r.Lock()
			if r.pendingP1bRelay > 0 {
				// this is a ballot we have not seen... and have not relayed before
				// so we can reply nack to any outstanding p1a relays
				log.Debugf("Short circuiting p1a relay. previous ballot=%v, new ballot=%v", oldBallot, m.Ballot)
				r.Send(oldBallot.ID(), m)
			}
			r.pendingP1bRelay = time.Now().UnixNano()
			r.p1bRelayRoutedMsg = &RoutedMsg{Progress: routedMsg.Progress, Hops: routedMsg.Hops, Payload: make([]P1b, 0)}
			needToPropagate = true
			r.Unlock()
		} else {
			log.Debugf("Stopping relay as we reached max depth")
		}
	} else {
		log.Debugf("Node %v received P1a with ballot %v, however, a newer or same ballot %v is known. Not relaying.", r.ID(), m.Ballot, oldBallot)
	}

	if routedMsg.Progress+1 == r.maxDepth {
		r.HandleP1a(m, routedMsg.GetLastProgressHop())
	} else {
		r.HandleP1a(m, r.ID())
	}

	return needToPropagate
}

//func (r *Replica) handleP2a(m P2a) {
//	log.Debugf("Node %v handling msg {%v}", r.ID(), m)
//	r.HandleP2a(m, m.Ballot.ID())
//}

func (r *Replica) handleP2aRelay(m P2a, routedMsg RoutedMsg) bool {
	log.Debugf("Node %v handling msg {%v}", r.ID(), m)
	if routedMsg.Progress+1 == r.maxDepth {
		r.HandleP2a(m, routedMsg.GetLastProgressHop())
	} else {
		// we are not at the leaf level yet, so need to have a relay setup
		r.Lock()
		if _, ok := r.p2bRelaysMapByBalSlot[m.Slot]; !ok {
			r.newP2bRelay(m, routedMsg)
		} else {
			// we have this slot already. Check ballot. if we were collecting responses for lesser ballot,
			// we can reply nack to old sender with such lesser ballot.
			// if we were collecting responses for higher ballot reply nack to new sender
			p2b := r.p2bRelaysMapByBalSlot[m.Slot].Payload.(P2b)
			if p2b.Ballot < m.Ballot {
				nackP2b := &P2b{Ballot: m.Ballot, Slot: m.Slot, ID: make([]paxi.ID, 0)}
				r.Send(p2b.Ballot.ID(), nackP2b)
				r.newP2bRelay(m, routedMsg)
			} else if p2b.Ballot > m.Ballot {
				nackP2b := &P2b{Ballot: p2b.Ballot, Slot: m.Slot, ID: make([]paxi.ID, 0)}
				r.Send(p2b.Ballot.ID(), nackP2b)
			}
		}
		r.Unlock()
		// self loop
		r.HandleP2a(m, r.ID())
	}
	return true
}

func (r *Replica) newP2bRelay(m P2a, routedMsg RoutedMsg) {
	routedP2b := RoutedMsg{
		Hops:      routedMsg.Hops,
		IsForward: false,
		Progress:  routedMsg.Progress,
		Payload:   P2b{Ballot: m.Ballot, Slot: m.Slot, ID: make([]paxi.ID, 0)},
	}

	r.p2bRelaysMapByBalSlot[m.Slot] = &routedP2b
	r.p2bRelaysTimeMapByBalSlot[m.Slot] = time.Now().UnixNano()
}

func (r *Replica) handleP3(m P3) bool {
	log.Debugf("Node %v handling msg {%v}", r.ID(), m)
	r.HandleP3(m)
	return true
}

//*********************************************************************************************************************
// Reply Propagation
//*********************************************************************************************************************

//***************
// P1
//***************
func (r *Replica) handleP1bLeader(p1bs []P1b) {
	log.Debugf("Node %v received aggregated P1b {%v}", r.ID(), p1bs)
	for _, p1b := range p1bs {
		r.HandleP1b(p1b)
	}
}

func (r *Replica) handleP1b(m P1b) {
	if r.pendingP1bRelay > 0 && r.p1bRelayRoutedMsg != nil {
		r.handleP1bRelay(m) // received p1b from leaf, aggregate it
	} else {
		log.Debugf("Node %v received P1b {%v}", r.ID(), m)
		r.HandleP1b(m)
	}
}

func (r *Replica) handleP1bRelay(m P1b) {
	r.Lock()
	defer r.Unlock()
	// we are just relaying this message
	log.Debugf("Node %v received P1b for relay {%v}", r.ID(), m)
	//r.p1bRelays = append(r.p1bRelays, m)
	r.p1bRelayRoutedMsg.Payload = append(r.p1bRelayRoutedMsg.Payload.([]P1b), m)
	if r.readyToRelayP1b(m.Ballot, r.p1bRelayDepth) {

		log.Debugf("Relaying p1bs {%v} to %v", r.p1bRelayRoutedMsg.Payload, m.Ballot.ID())
		// relay RoutedMsg downstream unless relaying back to root, in which case just sand []P1b to ease processing at leader
		if r.p1bRelayRoutedMsg.Progress == 0 {
			r.Send(r.p1bRelayRoutedMsg.GetLastProgressHop(), r.p1bRelayRoutedMsg.Payload)
		} else {
			r.Send(r.p1bRelayRoutedMsg.GetLastProgressHop(), r.p1bRelayRoutedMsg)
		}
		r.pendingP1bRelay = 0
		r.p1bRelayDepth = 0
		r.p1bRelayRoutedMsg = nil
	}
}

func (r *Replica) readyToRelayP1b(ballot paxi.Ballot, depth uint8) bool {
	pgToRelay := r.relayGroups[r.myRelayGroup]
	p1bs := r.p1bRelayRoutedMsg.Payload.([]P1b)
	log.Debugf("Now have %d messages to relay for p1b Ballot %v. PeerGroup to relay is %d nodes at depth %d", len(p1bs), ballot, len(pgToRelay.nodes), depth)
	if len(p1bs) == len(pgToRelay.nodes)/2+1 {
		return true
	}
	if len(p1bs) == len(pgToRelay.nodes)/2 {
		for _, id := range pgToRelay.nodes {
			if id == ballot.ID() {
				return true
			}
		}
	}
	return false
	//return true
}

//***************
// P2
//***************

func (r *Replica) handleP2b(m P2b) {
	if r.IsLeader() {
		// we received p2b aggregated reply, so just handle it at the pigpaxos level
		r.HandleP2b(m.Slot, m.Ballot, m.ID)
	} else {
		// here we handle the P2b coming from the leaf node
		// or the rare case of P2b coming to node who was a leader but not anymore.
		// in the latter case, we are ok to just ignore
		r.handleP2bRelay(m)
	}
}

func (r *Replica) handleP2bAggregated(m P2bAggregated) {
	log.Debugf("Handling P2bAggregated: %v", m)
	if r.IsLeader() {
		r.UpdateLastExecuteByNode(m.RelayID, m.RelayLastExecute)
		// we received p2b aggregated reply, so just handle it at the pigpaxos level
		if m.MissingIDs != nil && len(m.MissingIDs) > 0 {
			ids := make([]paxi.ID, len(r.relayGroups[r.NodeIdsToGroup[m.RelayID]].nodes))
			copy(ids, r.relayGroups[r.NodeIdsToGroup[m.RelayID]].nodes)
			for _, missingId := range m.MissingIDs {
				for i, id := range ids {
					if id == missingId {
						ids[i] = ids[len(ids)-1]
						ids[len(ids)-1] = 0
						ids = ids[:len(ids)-1]
						break
					}
				}
			}
			log.Debugf("Calling HandleP2b with ids: %v", ids)
			r.HandleP2b(m.Slot, m.Ballot, ids)
		} else {
			log.Debugf("Calling HandleP2b with ids: %v", r.relayGroups[r.NodeIdsToGroup[m.RelayID]].nodes)
			r.HandleP2b(m.Slot, m.Ballot, r.relayGroups[r.NodeIdsToGroup[m.RelayID]].nodes)
		}
	} else {
		log.Errorf("Can process this type of messages only on leader node: %v", m)
	}
}

func (r *Replica) handleP2bRelay(m P2b) {
	// we are just relaying this message
	r.RLock()
	p2bForRelay, haveSlot := r.p2bRelaysMapByBalSlot[m.Slot]
	r.RUnlock()
	log.Debugf("Node %v received P2b for relay {%v}", r.ID(), m)
	if !haveSlot {
		log.Debugf("Unknown P2b {%v} Ballot to relay. It may have already been replied", m)
	} else {
		p2b := p2bForRelay.Payload.(P2b)
		if p2b.Ballot == m.Ballot {
			p2b.ID = append(p2b.ID, m.ID...)
			r.Lock()
			p2bForRelay.Payload = p2b
			r.Unlock()
			log.Debugf("Now have %d messages to relay for p2b Slot %d Ballot %v", len(p2b.ID), m.Slot, m.Ballot)
			if r.readyToRelayP2b(m.Slot) {
				var missingIds []paxi.ID
				//if r.relaySlack > 0 {
				//	missingIds = r.computeMissingIDsForP2b(p2b)
				//} else {
				//	missingIds = make([]paxi.ID, 0)
				//}
				//执行过程先进行relay下的选票收集，后计算需要跳过的节点
				missingIds = r.computeMissingIDsForP2b(p2b)
				log.Debugf("Relaying p2bs {%v} to %v", p2bForRelay, m.Ballot.ID())
				// relay RoutedMsg downstream unless relaying back to root
				if p2bForRelay.Progress == 0 {
					if *useSmallP2b {
						p2bSmall := P2bAggregated{
							Ballot:           m.Ballot,
							Slot:             m.Slot,
							RelayLastExecute: r.execute - 1,
							MissingIDs:       missingIds,
							RelayID:          r.ID()}
						r.Send(p2bForRelay.GetLastProgressHop(), p2bSmall)
					} else {
						r.Send(p2bForRelay.GetLastProgressHop(), p2bForRelay.Payload)
					}
				} else {
					r.Send(p2bForRelay.GetLastProgressHop(), p2bForRelay)
				}
				r.Lock()
				delete(r.p2bRelaysMapByBalSlot, m.Slot)
				delete(r.p2bRelaysTimeMapByBalSlot, m.Slot)
				r.Unlock()
			}
		} else {
			// this is normal, when follower has already received newer ballot and replies with it to let
			// this node know of higher ballot
			log.Errorf("Node %v received P2b to reply with non-matching ballot (%v) to the relay ballot (%v)",
				r.ID(), m.Ballot, p2b.Ballot)
			nackP2b := &P2b{Ballot: m.Ballot, Slot: m.Slot, ID: make([]paxi.ID, 0)}
			r.Send(p2b.Ballot.ID(), nackP2b)
		}
	}
}

func (r *Replica) computeMissingIDsForP2b(p2b P2b) []paxi.ID {
	//take a copy of this group
	missingIds := make([]paxi.ID, len(r.relayGroups[r.myRelayGroup].nodes))
	copy(missingIds, r.relayGroups[r.myRelayGroup].nodes)
	for _, id := range p2b.ID {
		for i, missingId := range missingIds {
			if id == missingId {
				missingIds[i] = missingIds[len(missingIds)-1]
				missingIds[len(missingIds)-1] = 0
				missingIds = missingIds[:len(missingIds)-1]
				break
			}
		}
	}
	return missingIds
}

func (r *Replica) readyToRelayP2b(m int) bool {
	r.RLock()
	defer r.RUnlock()
	if r.p2bRelaysMapByBalSlot[m] == nil {
		return false
	}
	pgToRelay := r.relayGroups[r.myRelayGroup]
	p2b := r.p2bRelaysMapByBalSlot[m].Payload.(P2b)
	if len(p2b.ID) == len(pgToRelay.nodes)/2+1 {
		return true
	}
	if len(p2b.ID) == len(pgToRelay.nodes)/2 {
		if r.NodeIdsToGroup[p2b.Ballot.ID()] == r.myRelayGroup {
			return true
		}
	}
	return false
}

//*********************************************************************************************************************
// Client Request Handling
//*********************************************************************************************************************

func (r *Replica) handleRequest(m paxi.Request) {
	log.Debugf("Replica %s received %v\n", r.ID(), m)

	if !*stableLeader || r.LayerPaxos.IsLeader() || r.LayerPaxos.Ballot() == 0 {
		r.LayerPaxos.HandleRequest(m)
	} else {
		go r.Forward(r.LayerPaxos.Leader(), m)
	}
}
