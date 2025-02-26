package paxos

import (
	"flag"
	"strconv"
	"time"

	"pigpaxos"
	"pigpaxos/log"
)

var ephemeralLeader = flag.Bool("ephemeral_leader", false, "unstable leader, if true paxos replica try to become leader instead of forward requests to current leader")
var read = flag.String("read", "", "read from \"leader\", \"quorum\" or \"any\" replica")

const (
	HTTPHeaderSlot       = "Slot"
	HTTPHeaderBallot     = "Ballot"
	HTTPHeaderExecute    = "Execute"
	HTTPHeaderInProgress = "Inprogress"
)

// Replica for one Paxos instance
type Replica struct {
	paxi.Node
	cleanupMultiplier uint64
	*Paxos
}

// NewReplica generates new Paxos replica
func NewReplica(id paxi.ID) *Replica {
	r := new(Replica)
	r.Node = paxi.NewNode(id)
	r.Paxos = NewPaxos(r)
	r.cleanupMultiplier = 3
	r.Register(paxi.Request{}, r.handleRequest)
	r.Register(P1a{}, r.HandleP1a)
	r.Register(P1b{}, r.HandleP1b)
	r.Register(P2a{}, r.HandleP2a)
	r.Register(P2b{}, r.HandleP2b)
	r.Register(P3{}, r.HandleP3)
	r.Register(P3RecoverRequest{}, r.HandleP3RecoverRequest)
	r.Register(P3RecoverReply{}, r.HandleP3RecoverReply)

	go r.startTicker()

	return r
}

//*********************************************************************************************************************
// Timer for all timed events, such as timeouts and log clean ups
//*********************************************************************************************************************
func (r *Replica) startTicker() {
	var ticks uint64 = 0
	for now := range time.Tick(10 * time.Millisecond) {
		// log cleanup
		ticks++
		if ticks%r.cleanupMultiplier == 0 {
			r.CleanupLog()
		}

		if r.IsLeader() {
			r.P3Sync(now.UnixNano() / int64(time.Millisecond))
		} else {
			// check if RSM on follower is stuck and recover stuck slots. It may get stuck due to network failures (dropped messages)
			// Messages my drop in PigPaxos due to relay faults
			r.CheckNeedForRecovery()
		}
	}
}

func (r *Replica) handleRequest(m paxi.Request) {
	log.Debugf("Replica %s received %v\n", r.ID(), m)

	if m.Command.IsRead() && *read != "" {
		v, inProgress := r.readInProgress(m)
		reply := paxi.Reply{
			Command:    m.Command,
			Value:      v,
			Properties: make(map[string]string),
			Timestamp:  time.Now().Unix(),
		}
		reply.Properties[HTTPHeaderSlot] = strconv.Itoa(r.Paxos.slot)
		reply.Properties[HTTPHeaderBallot] = r.Paxos.ballot.String()
		reply.Properties[HTTPHeaderExecute] = strconv.Itoa(r.Paxos.execute - 1)
		reply.Properties[HTTPHeaderInProgress] = strconv.FormatBool(inProgress)
		m.Reply(reply)
		return
	}

	if *ephemeralLeader || r.Paxos.IsLeader() || r.Paxos.Ballot() == 0 {
		r.Paxos.HandleRequest(m)
	} else {
		go r.Forward(r.Paxos.Leader(), m)
	}
}

func (r *Replica) readInProgress(m paxi.Request) (paxi.Value, bool) {
	// TODO
	// (1) last slot is read?
	// (2) entry in log over writen
	// (3) value is not overwriten command

	// is in progress
	for i := r.Paxos.slot; i >= r.Paxos.execute; i-- {
		entry, exist := r.Paxos.log[i]
		if exist && entry.command.Key == m.Command.Key {
			return entry.command.Value, true
		}
	}

	// not in progress key
	return r.Node.Execute(m.Command), false
}
