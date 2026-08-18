package work

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Priority int

const (
	PriorityVisible Priority = iota
	PriorityInteractive
	PriorityPrefetch
	PriorityBackground
	PriorityIdle
)

type Progress struct {
	Completed int64
	Total     int64
	Message   string
	Units     string
}

type Reporter interface {
	Report(Progress)
}

type reporterFunc func(Progress)

func (f reporterFunc) Report(progress Progress) { f(progress) }

type Task struct {
	Key      string
	Priority Priority
	Kind     string
	Run      func(context.Context, Reporter) error
}

type Snapshot struct {
	ID         uint64
	Key        string
	Priority   Priority
	Kind       string
	State      string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	Progress   Progress
}

type Handle struct {
	task *scheduledTask
}

func (h *Handle) Done() <-chan struct{} { return h.task.done }

func (h *Handle) Err() error {
	<-h.task.done
	h.task.mu.RLock()
	defer h.task.mu.RUnlock()
	return h.task.err
}

func (h *Handle) Cancel() {
	h.task.cancel()
}

func (h *Handle) Snapshot() Snapshot {
	return h.task.snapshot()
}

type scheduledTask struct {
	mu       sync.RWMutex
	id       uint64
	task     Task
	sequence uint64
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}

	state      string
	createdAt  time.Time
	startedAt  time.Time
	finishedAt time.Time
	progress   Progress
	err        error
	finished   bool
}

func (t *scheduledTask) snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return Snapshot{
		ID:         t.id,
		Key:        t.task.Key,
		Priority:   t.task.Priority,
		Kind:       t.task.Kind,
		State:      t.state,
		CreatedAt:  t.createdAt,
		StartedAt:  t.startedAt,
		FinishedAt: t.finishedAt,
		Progress:   t.progress,
	}
}

type Scheduler struct {
	mu       sync.Mutex
	cond     *sync.Cond
	queue    []*scheduledTask
	keyed    map[string]*scheduledTask
	closed   bool
	nextID   atomic.Uint64
	sequence atomic.Uint64

	listenersMu sync.RWMutex
	listeners   map[uint64]func(Snapshot)
	nextSubID   atomic.Uint64
}

func NewScheduler(concurrency int) *Scheduler {
	if concurrency <= 0 {
		concurrency = 1
	}

	s := &Scheduler{
		keyed:     make(map[string]*scheduledTask),
		listeners: make(map[uint64]func(Snapshot)),
	}
	s.cond = sync.NewCond(&s.mu)

	for index := 0; index < concurrency; index++ {
		go s.worker()
	}
	return s
}

var Default = NewScheduler(8)

func (s *Scheduler) Submit(parent context.Context, task Task) *Handle {
	if parent == nil {
		parent = context.Background()
	}
	if task.Run == nil {
		panic("work: nil task Run")
	}
	if task.Kind == "" {
		task.Kind = "default"
	}

	s.mu.Lock()
	if task.Key != "" {
		if existing := s.keyed[task.Key]; existing != nil {
			existing.mu.RLock()
			active := !existing.finished
			existing.mu.RUnlock()
			if active {
				s.mu.Unlock()
				return &Handle{task: existing}
			}
		}
	}

	ctx, cancel := context.WithCancel(parent)
	item := &scheduledTask{
		id:        s.nextID.Add(1),
		task:      task,
		sequence:  s.sequence.Add(1),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
		state:     "queued",
		createdAt: time.Now(),
	}
	s.queue = append(s.queue, item)
	if task.Key != "" {
		s.keyed[task.Key] = item
	}
	s.sortQueueLocked()
	s.mu.Unlock()

	s.publish(item.snapshot())
	s.cond.Signal()
	return &Handle{task: item}
}

func (s *Scheduler) Subscribe(listener func(Snapshot)) func() {
	if listener == nil {
		return func() {}
	}
	id := s.nextSubID.Add(1)
	s.listenersMu.Lock()
	s.listeners[id] = listener
	s.listenersMu.Unlock()

	return func() {
		s.listenersMu.Lock()
		delete(s.listeners, id)
		s.listenersMu.Unlock()
	}
}

func (s *Scheduler) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	queued := append([]*scheduledTask(nil), s.queue...)
	s.queue = nil
	s.mu.Unlock()

	for _, task := range queued {
		task.cancel()
		s.finish(task, context.Canceled, "cancelled")
	}
	s.cond.Broadcast()
}

func (s *Scheduler) worker() {
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.closed {
			s.cond.Wait()
		}
		if s.closed {
			s.mu.Unlock()
			return
		}
		item := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()

		if err := item.ctx.Err(); err != nil {
			s.finish(item, err, "cancelled")
			continue
		}

		item.mu.Lock()
		item.state = "running"
		item.startedAt = time.Now()
		item.mu.Unlock()
		s.publish(item.snapshot())

		reporter := reporterFunc(func(progress Progress) {
			item.mu.Lock()
			if !item.finished {
				item.progress = progress
			}
			item.mu.Unlock()
			s.publish(item.snapshot())
		})

		err := item.task.Run(item.ctx, reporter)
		state := "completed"
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(item.ctx.Err(), context.Canceled) {
				state = "cancelled"
			} else {
				state = "failed"
			}
		}
		s.finish(item, err, state)
	}
}

func (s *Scheduler) finish(item *scheduledTask, err error, state string) {
	item.mu.Lock()
	if item.finished {
		item.mu.Unlock()
		return
	}
	item.finished = true
	item.state = state
	item.err = err
	item.finishedAt = time.Now()
	close(item.done)
	item.mu.Unlock()

	s.mu.Lock()
	if item.task.Key != "" && s.keyed[item.task.Key] == item {
		delete(s.keyed, item.task.Key)
	}
	s.mu.Unlock()

	s.publish(item.snapshot())
}

func (s *Scheduler) sortQueueLocked() {
	sort.SliceStable(s.queue, func(i, j int) bool {
		left := s.queue[i]
		right := s.queue[j]
		if left.task.Priority != right.task.Priority {
			return left.task.Priority < right.task.Priority
		}
		return left.sequence < right.sequence
	})
}

func (s *Scheduler) publish(snapshot Snapshot) {
	s.listenersMu.RLock()
	listeners := make([]func(Snapshot), 0, len(s.listeners))
	for _, listener := range s.listeners {
		listeners = append(listeners, listener)
	}
	s.listenersMu.RUnlock()

	for _, listener := range listeners {
		listener(snapshot)
	}
}
