package raft

import (
	"fmt"
	"strings"
)

type Log struct {
	Entries []Entry
	Index0  int
	LastLog Entry
}

type Entry struct {
	Command interface{}
	Term    int
	Index   int
}

func (l *Log) append(entries ...Entry) {
	l.Entries = append(l.Entries, entries...)
	l.LastLog = l.Entries[l.len()-1]
}

func makeEmptyLog() Log {
	log := Log{
		Entries: make([]Entry, 0),
		Index0:  0,
	}
	return log
}

func (l *Log) at(idx int) *Entry {
	return &l.Entries[idx]
}

func (l *Log) truncate(idx int) {
	l.Entries = l.Entries[:idx]
	l.LastLog = l.Entries[l.len()-1]
}

func (l *Log) slice(idx int) []Entry {
	return l.Entries[idx:]
}

func (l *Log) len() int {
	return len(l.Entries)
}

func (e *Entry) String() string {
	return fmt.Sprint(e.Term)
}

func (l *Log) String() string {
	nums := []string{}
	for _, entry := range l.Entries {
		nums = append(nums, fmt.Sprintf("%4d", entry.Term))
	}
	return fmt.Sprint(strings.Join(nums, "|"))
}
