package sheetstocsv

type FileWorker struct {
	job chan interface{}
	res chan error
}

func NewWorkerPerAll(jobCount int, resCount int) FileWorker {
	return FileWorker{
		job: make(chan interface{}, jobCount),
		res: make(chan error, resCount),
	}
}
func NewWorker(jobCount int) FileWorker {
	return FileWorker{
		job: make(chan interface{}, jobCount),
		res: make(chan error, jobCount),
	}
}

func (w *FileWorker) WaitAll(fn func(error)) {
	for res := range w.res {
		fn(res)
	}
}
