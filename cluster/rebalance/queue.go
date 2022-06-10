package rebalance

type queue[T any] struct {
	values []T
}

func newQueue[T any](cap uint32) *queue[T] {
	return &queue[T]{
		values: make([]T, 0, cap),
	}
}

func (s *queue[T]) Enqueue(value T) {
	s.values = append(s.values, value)
}

func (s *queue[T]) Dequeue() (T, bool) {
	var output T
	if len(s.values) == 0 {
		return output, false
	}
	// get the last item
	highIndex := len(s.values) - 1
	output = s.values[highIndex]
	s.values = s.values[:highIndex]
	return output, true
}

func (s *queue[T]) Length() int {
	return len(s.values)
}
