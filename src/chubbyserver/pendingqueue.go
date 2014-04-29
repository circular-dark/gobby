package chubbyserver

import (
	"container/list"
	"github.com/gobby/src/command"
	"github.com/gobby/src/paxos"
	"sync"
//"fmt"
)

type Queue struct {
	l    *list.List
	lock sync.Mutex
	cond *sync.Cond
}

func NewQueue() *Queue {
	q := Queue{}
	q.l = list.New()
	q.cond = sync.NewCond(&(q.lock))
	return &q
}

func (q *Queue) Enqueue(c *command.Command) {
//fmt.Println("Enqueue command:"+c.ToString())
	q.lock.Lock()
	//q.cond.L.Lock()
	q.l.PushBack(c)
	q.lock.Unlock()
	//q.cond.L.Unlock()
	q.cond.Signal()
//fmt.Println("After Enqueue command:"+c.ToString())
}

func (q *Queue) Dequeue() *command.Command {
	q.lock.Lock()
	//q.cond.L.Lock()
	for q.l.Len() == 0 {
		q.cond.Wait()
	}
	p := q.l.Front()
	q.l.Remove(p)
	q.lock.Unlock()
	//q.cond.L.Unlock()
	return p.Value.(*command.Command)
}

func ReplicateRoutine(q *Queue, pn paxos.PaxosNode) {
//fmt.Println("in Replicate Routine")
	for {
		c := q.Dequeue()
//fmt.Println("Dequeue command:"+c.ToString())
		pn.Replicate(c)
	}
}
