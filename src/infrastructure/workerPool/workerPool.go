package worker_pool

type Task func()

type IWorkerPool interface {
	SubmitTask(task Task) error

	Running() int

	Capability() int

	Available() int

	Resize(size int)

	Restart()

	Shutdown()
}
