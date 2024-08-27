package raft

type InstallSnapshotArgs struct {
	Term              int
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Offset            int
	Data              []byte
	Done              bool
	FirstLog          Entry
}

type InstallSnapshotReply struct {
	Term int
}

func (rf *Raft) sendSnapshotToFollower(followerId int) {
	rf.mu.Lock()

	snapshot := rf.persister.ReadSnapshot()
	args := InstallSnapshotArgs{
		Term:              rf.currentTerm,
		LeaderId:          rf.me,
		LastIncludedIndex: rf.lastIncludedIndex,
		LastIncludedTerm:  rf.lastIncludedTerm,
		Offset:            0,
		Data:              snapshot,
		Done:              true,
		FirstLog:          *rf.log.at(rf.log.Index0),
	}
	rf.mu.Unlock()
	var reply InstallSnapshotReply
	if rf.sendInstallSnapshot(followerId, &args, &reply) {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		if reply.Term > rf.currentTerm {
			rf.setNewTerm(reply.Term)
			return
		}

		rf.nextIndex[followerId] = rf.lastIncludedIndex + 2
		rf.matchIndex[followerId] = rf.lastIncludedIndex + 1
	}
}
func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm

	if args.Term < rf.currentTerm {
		return
	}

	rf.setNewTerm(args.Term)
	rf.resetElectionTimer()

	if args.LastIncludedIndex <= rf.commitIndex {
		return
	}
	if rf.state == Candidate {
		rf.state = Follower
	}

	if args.LastIncludedIndex >= rf.log.LastLog.Index {
		rf.log = makeEmptyLog()
		rf.log.append(args.FirstLog)
		rf.log.Index0 = args.FirstLog.Index
	} else {
		rf.log.truncateFrom(args.LastIncludedIndex)
		rf.log.replaceIndex0(args.FirstLog)
	}
	rf.persist()
	rf.persister.Save(rf.persister.raftstate, args.Data)

	msg := ApplyMsg{
		CommandValid:  false,
		SnapshotValid: true,
		Snapshot:      args.Data,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	}
	rf.mu.Unlock()
	rf.applyCh <- msg
	rf.mu.Lock()
	rf.commitIndex = args.LastIncludedIndex
	rf.lastApplied = args.LastIncludedIndex
}
