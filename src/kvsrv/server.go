package kvsrv

import (
	"log"
	"sync"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type dupTable struct {
	seq   int
	value string
}

type KVServer struct {
	mu sync.Mutex

	// Your definitions here.
	data        map[string]string
	clientTable map[int64]*dupTable
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// duplicate detection
	// if client is new, create the map of clientid -> duplicateTable
	if kv.clientTable[args.ClientId] == nil {
		// 当这个client没有写入操作时，它读取的数据永远是当前的服务器Value
		reply.Value = kv.data[args.Key]
		return
	}

	dt := kv.clientTable[args.ClientId]

	// if seq exists
	if dt.seq == args.Seq {
		reply.Value = dt.value
		return
	}

	dt.seq = args.Seq
	dt.value = kv.data[args.Key]
	reply.Value = dt.value
	DPrintf("Server: ClientId %v Get reply %v and dt.seq is %v, value is %v\n",
		args.ClientId, args.Seq, dt.seq, dt.value)
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// duplicate detection
	// if client is new, create the map of clientid -> duplicateTable
	if kv.clientTable[args.ClientId] == nil {
		kv.data[args.Key] = args.Value
		return
		// kv.clientTable[args.ClientId] = &dupTable{-1, ""}
	}

	dt := kv.clientTable[args.ClientId]

	// if seq exists
	if dt.seq == args.Seq {
		return
	}

	dt.seq = args.Seq
	dt.value = ""
	kv.data[args.Key] = args.Value
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// duplicate detection
	// if client is new, create the map of clientid -> duplicateTable
	if kv.clientTable[args.ClientId] == nil {
		kv.clientTable[args.ClientId] = &dupTable{-1, ""}
	}

	dt := kv.clientTable[args.ClientId]
	DPrintf("Server: ClientId %v Append reply %v and dt.seq is %v\n",
		args.ClientId, args.Seq, dt.seq)

	// if seq exists
	if dt.seq == args.Seq {
		reply.Value = dt.value
		return
	}

	// return old value
	dt.seq = args.Seq
	dt.value = kv.data[args.Key]
	reply.Value = dt.value
	// append map[key]
	kv.data[args.Key] = kv.data[args.Key] + args.Value
	DPrintf("Server: ClientId %v Append reply %v and dt.seq is %v, value is %v\n",
		args.ClientId, args.Seq, dt.seq, dt.value)
}

func StartKVServer() *KVServer {
	kv := new(KVServer)

	// You may need initialization code here.
	kv.data = map[string]string{}
	kv.clientTable = map[int64]*dupTable{}

	return kv
}
