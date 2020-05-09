package utils

import "context"

func ORContext(contexts ...context.Context) context.Context {
	switch len(contexts) {
	case 0:
		return nil
	case 1:
		return contexts[0]
	}

	orDone, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		switch len(contexts) {
		case 2:
			select {
			case <-contexts[0].Done():
			case <-contexts[1].Done():
			}
		default:
			select {
			case <-contexts[0].Done():
			case <-contexts[1].Done():
			case <-contexts[2].Done():
			case <-ORContext(append(contexts[3:], orDone)...).Done():
			}
		}
	}()
	return orDone
}
