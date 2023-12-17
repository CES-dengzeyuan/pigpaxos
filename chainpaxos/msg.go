package chainpaxos

import (
	"encoding/gob"
	"fmt"
	"pigpaxos"
	"sync"
)

func init() {
	gob.Register(P1b{})
	gob.Register(P2b{})
	gob.Register(P2bAggregated{})
	gob.Register([]P1b{})
	gob.Register([]P2b{})
	gob.Register(P1a{})
	gob.Register(P2a{})
	gob.Register(P3{})
	gob.Register(P3RecoverRequest{})
	gob.Register(P3RecoverReply{})
	gob.Register(RoutedMsg{})

	gob.Register(P2aChain{})
}

// CommandBallot combines each command with its ballot number
type CommandBallot struct {
	Command paxi.Command
	Ballot  paxi.Ballot
}

func (cb CommandBallot) String() string {
	return fmt.Sprintf("cmd=%v b=%v", cb.Command, cb.Ballot)
}

type RoutedMsg struct {
	Hops      []paxi.ID
	IsForward bool
	Progress  uint8
	Payload   interface{}
	lock      sync.Mutex
}

func (m *RoutedMsg) GetLastProgressHop() paxi.ID {
	return m.Hops[m.Progress]
}

func (m *RoutedMsg) GetPreviousProgressHop() paxi.ID {
	return m.Hops[m.Progress-1]
}

func (m RoutedMsg) String() string {
	return fmt.Sprintf("RoutedMsg {Hops=%v IsForward=%v Progress=%v, Payload=%v}", m.Hops, m.IsForward, m.Progress, m.Payload)
}

type RoutedChainMsg struct {
	Hops      []paxi.ID
	IsForward bool
	Progress  uint8
	Payload   interface{}
	lock      sync.Mutex
}

func (m *RoutedChainMsg) GetLastProgressHop() paxi.ID {
	return m.Hops[m.Progress]
}

func (m *RoutedChainMsg) GetPreviousProgressHop() paxi.ID {
	return m.Hops[m.Progress-1]
}

func (m RoutedChainMsg) String() string {
	return fmt.Sprintf("RoutedChainMsg {Hops=%v IsForward=%v Progress=%v, Payload=%v}", m.Hops, m.IsForward, m.Progress, m.Payload)
}

// P1b promise message
type P1b struct {
	ID     paxi.ID // from node id
	Ballot paxi.Ballot
	Log    map[int]CommandBallot // uncommitted logs
}

func (m P1b) String() string {
	return fmt.Sprintf("P1b {b=%v id=%s log=%v}", m.Ballot, m.ID, m.Log)
}

// P2b accepted message
type P2b struct {
	ID     []paxi.ID // from node id
	Ballot paxi.Ballot
	Slot   int
}

func (m P2b) String() string {
	return fmt.Sprintf("P2b {b=%v id=%s s=%d}", m.Ballot, m.ID, m.Slot)
}

// P2b accepted message
type P2bAggregated struct {
	MissingIDs       []paxi.ID // node ids not collected by relay
	RelayID          paxi.ID
	RelayLastExecute int
	Ballot           paxi.Ballot
	Slot             int
}

func (m P2bAggregated) String() string {
	return fmt.Sprintf("P2b {b=%v RelayId=%s RelayLastExecute=%d s=%d, missingIDs=%v}", m.Ballot, m.RelayID, m.RelayLastExecute, m.Slot, m.MissingIDs)
}

// P1a prepare message
type P1a struct {
	Ballot paxi.Ballot
}

func (m P1a) String() string {
	return fmt.Sprintf("P1a {b=%v}", m.Ballot)
}

// P2a accept message
type P2a struct {
	Ballot        paxi.Ballot
	Slot          int
	GlobalExecute int
	Command       paxi.Command
	P3msg         P3
}

func (m P2a) String() string {
	return fmt.Sprintf("P2a {b=%v s=%d cmd=%v, p3Msg=%v}", m.Ballot, m.Slot, m.Command, m.P3msg)
}

type P2aChain struct {
	Ballot        paxi.Ballot
	Slot          int
	GlobalExecute int
	Command       paxi.Command
	P3msg         P3

	Hops          []paxi.ID //链式拓扑
	PeerGroupNode []paxi.ID //分组，r.relayGroups[r.myRelayGroup]可以直接获取
	exId          paxi.ID   //先驱节点
	nextId        paxi.ID   //后继节点
	msgCnt        int       //记录选票数
	msg           RoutedMsg
}

func (m P2aChain) String() string {
	return fmt.Sprintf("P2aChain {b=%v s=%d cmd=%v, p3Msg=%v, Hops=%v, PeerGroup=%v, exID=%s, nextID=%s, msgCnt"+
		"=%d}", m.Ballot, m.Slot, m.Command, m.P3msg, m.Hops, m.PeerGroupNode, m.exId, m.nextId, m.msgCnt)
}

// P3 commit message
type P3 struct {
	Ballot paxi.Ballot
	Slot   []int
	//Command paxi.Command
}

func (m P3) String() string {
	return fmt.Sprintf("P3 {b=%v slots=%d}", m.Ballot, m.Slot)
}

type P3RecoverRequest struct {
	Ballot paxi.Ballot
	Slot   int
	NodeId paxi.ID
}

func (m P3RecoverRequest) String() string {
	return fmt.Sprintf("P3RecoverRequest {b=%v slots=%d, nodeToRecover=%v}", m.Ballot, m.Slot, m.NodeId)
}

type P3RecoverReply struct {
	Ballot  paxi.Ballot
	Slot    int
	Command paxi.Command
}

func (m P3RecoverReply) String() string {
	return fmt.Sprintf("P3RecoverReply {b=%v slots=%d, cmd=%v}", m.Ballot, m.Slot, m.Command)
}
