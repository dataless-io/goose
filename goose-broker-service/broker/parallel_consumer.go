package broker

import "sync"

// ConsumerHandler processes each delivery.
type ConsumerHandler func(Delivery)

// ParallelConsumer dispatches in parallel while serializing per event ID.
type ParallelConsumer struct {
	mu        sync.Mutex
	activeIDs map[string]bool
	queues    map[string][]Delivery
	handler   ConsumerHandler
}

func NewParallelConsumer(handler ConsumerHandler) *ParallelConsumer {
	return &ParallelConsumer{
		activeIDs: make(map[string]bool),
		queues:    make(map[string][]Delivery),
		handler:   handler,
	}
}

func (p *ParallelConsumer) Consume(ch <-chan Delivery) {
	var wg sync.WaitGroup
	for delivery := range ch {
		p.mu.Lock()
		id := delivery.ID
		if p.activeIDs[id] {
			p.queues[id] = append(p.queues[id], delivery)
			p.mu.Unlock()
			continue
		}

		p.activeIDs[id] = true
		wg.Add(1)
		go func(first Delivery) {
			defer wg.Done()
			p.worker(first)
		}(delivery)
		p.mu.Unlock()
	}

	wg.Wait()
}

func (p *ParallelConsumer) worker(current Delivery) {
	for {
		p.handler(current)

		p.mu.Lock()
		queue := p.queues[current.ID]
		if len(queue) == 0 {
			delete(p.activeIDs, current.ID)
			delete(p.queues, current.ID)
			p.mu.Unlock()
			return
		}

		current = queue[0]
		p.queues[current.ID] = queue[1:]
		p.mu.Unlock()
	}
}
