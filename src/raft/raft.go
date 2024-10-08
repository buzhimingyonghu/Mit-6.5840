package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"bytes"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labgob"
	"6.5840/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 3D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 3D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}
type RaftState string

const (
	Follower  RaftState = "Follower"
	Candidate           = "Candidate"
	Leader              = "Leader"
)

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	state         RaftState
	appendEntryCh chan *Entry
	heartBeat     time.Duration
	electionTime  time.Time
	// Persistent state on all servers:
	currentTerm int
	votedFor    int
	log         Log

	// Volatile state on all servers:
	commitIndex int
	lastApplied int

	// Volatile state on leaders:
	nextIndex         []int
	matchIndex        []int
	lastIncludedIndex int
	lastIncludedTerm  int
	applyCh           chan ApplyMsg
	applyCond         *sync.Cond
}

// return currentTerm and whether this server
// believes it is the leader.

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (3C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
	DPrintVerbose("[%v]: STATE: %v", rf.me, rf.log.String())
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	data := w.Bytes()
	rf.persister.Save(data, rf.persister.snapshot)
}

// restore previously persisted state.
func (rf *Raft) readPersistRaftstate(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var logs Log
	if d.Decode(&currentTerm) != nil || d.Decode(&votedFor) != nil || d.Decode(&logs) != nil {
		log.Fatal("failed to read persist\n")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = logs
	}

}
func (rf *Raft) readPersistSnapshot(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var lastIncludedIndex int
	if d.Decode(&lastIncludedIndex) != nil {
		log.Fatalf("snapshot decode error")
		return
	} else {
		if lastIncludedIndex >= rf.log.Index0 && lastIncludedIndex < rf.log.LastLog.Index {
			// 裁剪日志：删除快照之前的日志条目
			rf.lastIncludedIndex = rf.log.at(lastIncludedIndex).Index
			rf.lastIncludedTerm = rf.log.at(lastIncludedIndex).Term
			rf.log.truncateFrom(lastIncludedIndex)
			// // 保存快照和当前的 Raft 状态
			rf.persist()
			rf.persister.Save(rf.persister.raftstate, data)
			rf.lastApplied = lastIncludedIndex
			rf.commitIndex = lastIncludedIndex
		} else {
			rf.lastApplied = rf.log.Index0 - 1
			rf.commitIndex = rf.log.Index0 - 1
		}

	}
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// 确保快照的索引有效，并且比当前日志的最早索引更靠后
	if index >= rf.log.Index0 && index < rf.log.LastLog.Index {
		// 裁剪日志：删除快照之前的日志条目
		rf.lastIncludedIndex = rf.log.at(index).Index
		rf.lastIncludedTerm = rf.log.at(index).Term
		rf.log.truncateFrom(index)
		// // 保存快照和当前的 Raft 状态
		rf.persist()
		rf.persister.Save(rf.persister.raftstate, snapshot)
	}

}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Leader {
		return -1, rf.currentTerm, false
	}
	index := rf.log.LastLog.Index + 1
	term := rf.currentTerm

	log := Entry{
		Command: command,
		Index:   index,
		Term:    term,
	}
	rf.log.append(log)
	rf.persist()
	rf.appendEntries(false)

	return index, term, true
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		time.Sleep(rf.heartBeat)
		rf.mu.Lock()
		if rf.state == Leader {
			rf.appendEntries(true)
		}
		if time.Now().After(rf.electionTime) {
			rf.leaderElection()
		}
		rf.mu.Unlock()
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.heartBeat = 50 * time.Millisecond
	rf.resetElectionTimer()

	rf.log = makeEmptyLog()
	rf.log.append(Entry{-1, 0, 0})
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))

	rf.applyCh = applyCh
	rf.applyCond = sync.NewCond(&rf.mu)

	// initialize from state persisted before a crash
	rf.readPersistRaftstate(persister.ReadRaftState())
	rf.readPersistSnapshot(persister.ReadSnapshot())

	// start ticker goroutine to start elections
	go rf.ticker()

	go rf.applier()
	return rf
}
func (rf *Raft) apply() {
	rf.applyCond.Broadcast()
}

func (rf *Raft) applier() {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	for !rf.killed() {
		// all server rule 1
		if rf.commitIndex > rf.lastApplied && rf.log.LastLog.Index > rf.lastApplied {
			rf.lastApplied++
			applyMsg := ApplyMsg{
				CommandValid: true,
				Command:      rf.log.at(rf.lastApplied).Command,
				CommandIndex: rf.lastApplied,
			}
			DPrintVerbose("[%v]: COMMIT %d: %v", rf.me, rf.lastApplied, rf.commits())
			rf.mu.Unlock()
			rf.applyCh <- applyMsg
			rf.mu.Lock()
		} else {
			rf.applyCond.Wait()
		}
	}
}

func (rf *Raft) commits() string {
	nums := []string{}
	for i := rf.log.Index0; i <= rf.lastApplied; i++ {
		nums = append(nums, fmt.Sprintf("%4d", rf.log.at(i).Command))
	}
	return fmt.Sprint(strings.Join(nums, "|"))
}
