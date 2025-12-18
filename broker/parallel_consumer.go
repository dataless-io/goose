package broker

import (
	"sync"
)

// ConsumerHandler is the function signature for processing deliveries
type ConsumerHandler func(Delivery)

// ParallelConsumer handles reading from a subscription and dispatching work
// to concurrent workers, ensuring serial execution for same-ID events.
type ParallelConsumer struct {
	mu        sync.Mutex
	activeIDs map[string]bool
	queues    map[string][]Delivery
	handler   ConsumerHandler
}

// NewParallelConsumer creates a new consumer
func NewParallelConsumer(handler ConsumerHandler) *ParallelConsumer {
	return &ParallelConsumer{
		activeIDs: make(map[string]bool),
		queues:    make(map[string][]Delivery),
		handler:   handler,
	}
}

// Consume starts reading from the channel and dispatching workers.
// It blocks until the channel is closed and all workers have finished.
func (p *ParallelConsumer) Consume(ch <-chan Delivery) {
	var wg sync.WaitGroup

	for delivery := range ch {
		p.mu.Lock()

		id := delivery.ID // Assuming Event has ID field populated

		if p.activeIDs[id] {
			// ID is busy, enqueue
			p.queues[id] = append(p.queues[id], delivery)
			p.mu.Unlock()
		} else {
			// ID is free, start worker
			p.activeIDs[id] = true
			wg.Add(1)
			go func(first Delivery) {
				defer wg.Done()
				p.worker(first)
			}(delivery)
			p.mu.Unlock()
		}
	}

	wg.Wait()
}

// worker processes the given delivery and then drains the queue for that ID
func (p *ParallelConsumer) worker(initial Delivery) {
	current := initial

	for {
		// 1. Process the event (user logic)
		p.handler(current)

		// 2. Check for more work for this ID
		p.mu.Lock()
		queue := p.queues[current.ID]
		if len(queue) == 0 {
			// No more work, release ID
			delete(p.activeIDs, current.ID)
			delete(p.queues, current.ID) // cleanup empty slice
			p.mu.Unlock()
			return
		}

		// Pop next item
		current = queue[0]
		p.queues[current.ID] = queue[1:]
		p.mu.Unlock()
	}
}
