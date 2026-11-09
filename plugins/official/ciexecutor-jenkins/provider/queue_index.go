package provider

import "sync"

// maxRemembered bounds the queue items one provider process remembers; the oldest is dropped first.
const maxRemembered = 1024

// queueIndex remembers, for runs this process has seen in the Jenkins queue, the id of their queue item. The operation id does not
// carry it, and Jenkins keeps an item that left the queue only for a few minutes at /queue/item/<id>: with the id a lookup can tell
// what became of a run that is in neither the queue nor the job's builds (see impl.inGap). It is a hint, never a source of truth:
// a process that never saw the item, or lost it on restart, falls back to the lists, and an answer from Jenkins is always checked.
type queueIndex struct {
	mu    sync.Mutex
	ids   map[string]int64
	order []string
}

func (q *queueIndex) put(op string, id int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ids == nil {
		q.ids = map[string]int64{}
	}
	if _, ok := q.ids[op]; !ok {
		q.order = append(q.order, op)
	}
	q.ids[op] = id
	for len(q.order) > maxRemembered {
		delete(q.ids, q.order[0])
		q.order = q.order[1:]
	}
}

func (q *queueIndex) get(op string) (int64, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	id, ok := q.ids[op]
	return id, ok
}

func (q *queueIndex) forget(op string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.ids[op]; !ok {
		return
	}
	delete(q.ids, op)
	for i, o := range q.order {
		if o == op {
			q.order = append(q.order[:i], q.order[i+1:]...)
			break
		}
	}
}
