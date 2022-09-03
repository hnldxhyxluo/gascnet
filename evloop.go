package gascnet

func (this *evloop) doasync() {
	calls := this.asyncqueue.pullall()
	for _, call := range calls {
		call(this.id)
	}
}
