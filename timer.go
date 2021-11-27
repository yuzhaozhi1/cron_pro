package cron_pro

import (
	"sync"
)

// Timer 定义 定时器 接口
type Timer interface {
	AddTaskByFunc(taskName string, spec interface{}, task func()) (EntryID, error)
	AddTaskByJob(taskName string, spec interface{}, job interface{ Run() }) (EntryID, error)
	FindCron(taskName string) (*Cron, bool)
	StartTask(taskName string)
	StopTask(taskName string)
	Remove(taskName string, id int)
	Clear(taskName string)
	Close()
}

// TimerList 定时任务管理
type timer struct {
	taskList map[string]*Cron // 任务map map[任务名称]任务对象
	look     sync.Mutex            // 锁
}

// NewTimerTask 创建一个 新的Timer
func NewTimerTask() Timer {
	return &timer{taskList: make(map[string]*Cron)}
}

// AddTaskByFunc 通过函数的方法添加任务
func (t *timer) AddTaskByFunc(taskName string, spec interface{}, task func()) (EntryID, error) {
	t.look.Lock() // 加锁保护任务map的并发读写
	defer t.look.Unlock()

	// 如果没有这个任务就创建一个新的任务
	if _, ok := t.taskList[taskName]; !ok {
		t.taskList[taskName] = NewCron()
	}
	// AddFunc向Cron添加一个函数，以便按照给定的时间表运行。
	// 规范是使用此Cron实例的时区作为默认时区进行解析的。
	// 返回一个不透明的ID，以后可以使用该ID将其删除。
	id, err := t.taskList[taskName].AddFunc(spec, task)
	t.taskList[taskName].Start() // 在自己的goroutine中启动cron调度程序，如果已经启动，则不执行任何操作。
	return id, err
}

// AddTaskByJob 通过接口的方法添加任务
func (t *timer) AddTaskByJob(taskName string, spec interface{}, job interface{ Run() }) (EntryID, error) {
	t.look.Lock()
	defer t.look.Unlock()

	if _, ok := t.taskList[taskName]; !ok {
		t.taskList[taskName] = NewCron()
	}

	// AddJob 将job添加到 Cron 以按给定计划运行。
	id, err := t.taskList[taskName].AddJob(spec, job)
	t.taskList[taskName].Start()
	return id, err
}

// FindCron 获取对应taskName 的corn, 可能会为空
func (t *timer) FindCron(taskName string) (*Cron, bool) {
	t.look.Lock()
	v, ok := t.taskList[taskName]
	t.look.Unlock()
	return v, ok
}

// StartTask 开始任务
func (t *timer) StartTask(taskName string) {
	t.look.Lock()
	defer t.look.Unlock()
	if v, ok := t.taskList[taskName]; ok {
		v.Start()
	}
	return
}

// StopTask 停止任务
func (t *timer) StopTask(taskName string) {
	t.look.Lock()
	defer t.look.Unlock()
	if v, ok := t.taskList[taskName]; ok {
		v.Stop()
	}
	return
}

// Remove 根据taskName 删除指定任务
func (t *timer) Remove(taskName string, id int) {
	t.look.Lock()
	defer t.look.Unlock()
	if v, ok := t.taskList[taskName]; ok {
		v.Remove(EntryID(id))
	}
	return
}

// Clear 清除任务
func (t *timer) Clear(taskName string) {
	t.look.Lock()
	defer t.look.Unlock()
	if v, ok := t.taskList[taskName]; ok {
		v.Stop()
		delete(t.taskList, taskName)
	}
}

// Close 释放资源
func (t *timer) Close() {
	t.look.Lock()
	defer t.look.Unlock()
	for _, v := range t.taskList {
		v.Stop()
	}
}


