package cron_pro

// 循环链表的实现

// JobWrapper 任务包装
type JobWrapper func(Job) Job

// Chain 时间链表
type Chain struct {
	wrappers []JobWrapper
}

// NewChain 返回一个行的时间链表
func NewChain(c ...JobWrapper) Chain {
	return Chain{c}
}

// Then 用链中的所有JobWrappers装饰给定的作业。
func (c Chain) Then(j Job) Job {
	for i := range c.wrappers {
		j = c.wrappers[len(c.wrappers)-i-1](j)
	}
	return j
}
