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
	l.Index0 = l.Entries[0].Index
}

func makeEmptyLog() Log {
	log := Log{
		Entries: make([]Entry, 0),
		Index0:  0,
	}
	return log
}

func (l *Log) at(idx int) *Entry {
	return &l.Entries[idx-l.Index0]
}

func (l *Log) truncate(idx int) {
	l.Entries = l.Entries[:idx-l.Index0]
	l.LastLog = l.Entries[l.len()-1]
}
func (l *Log) truncateFrom(idx int) {
	l.Entries = l.Entries[idx+1-l.Index0:]
	l.Index0 = idx + 1
	l.LastLog = l.Entries[l.len()-1]
}
func (l *Log) replaceIndex0(e Entry) {
	l.Entries[0] = e
	l.LastLog = l.Entries[l.len()-1]
	l.Index0 = e.Index
}
func (l *Log) slice(idx int) []Entry {
	return l.Entries[idx-l.Index0:]
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
