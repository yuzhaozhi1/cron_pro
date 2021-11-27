package cron_pro

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"
)

// Job 提交cron 任务的interface
type Job interface {
	Run()
}

// Schedule 计时器
type Schedule interface {
	Next(time.Time) time.Time
}

// ------------Entry 相关

type EntryID int

// Entry 就是一个时间环,然后每个时间点上的func 组成
// 利用时间轮询算法
type Entry struct {
	ID         EntryID
	Schedule   Schedule  // 计时器
	Next       time.Time // 下次执行时间
	Prev       time.Time // 上次执行时间
	WrappedJob Job
	Job        Job // 任务
}

// ScheduleParser 用于定义解析时间cron 或者 解析 时间类型
type ScheduleParser interface {
	Parse(spec interface{}) (Schedule, error)
}

// byTime 需要可以用sort 进行排序, 所以需要实现 Len Swap 和 Less 方法
type byTime []*Entry

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}

// --------------- Cron

// Cron cron 核心结构体
type Cron struct {
	entries  []*Entry // 任务, 放在换行链表中
	chain    Chain
	stop     chan struct{}     // 停止的channel, 放值就停止
	add      chan *Entry       // 添加新任务
	remove   chan EntryID      // 删除一个任务
	snapshot chan chan []Entry // 复制一份任务链表
	running  bool              // 是否在运行,标志位
	// logger    Logger
	runningMu sync.Mutex
	location  *time.Location
	parser    ScheduleParser
	nextID    EntryID
	jobWaiter sync.WaitGroup
}

func NewCron() *Cron {
	c := &Cron{
		entries:   nil,
		chain:     NewChain(),
		add:       make(chan *Entry),
		stop:      make(chan struct{}),
		snapshot:  make(chan chan []Entry),
		remove:    make(chan EntryID),
		running:   false,
		runningMu: sync.Mutex{},
		// logger:    DefaultLogger,
		location: time.Local,
		parser:   standardParser,
	}
	// todo: 可以考虑添加 选项设计模式用于添加参数
	return c
}

// Run cron 运行
func (c *Cron) Run() {
	c.runningMu.Lock()
	if c.running {
		c.runningMu.Unlock()
		return
	}
	c.running = true
	c.runningMu.Unlock()
	c.run()
}

// Start 在它自己的goroutine中启动cron调度程序，如果已经启动则不执行任何操作
func (c *Cron) Start() {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

// RemoveTask 删除掉队列中的一个任务
func (c *Cron) RemoveTask(id int) {
	c.remove <- EntryID(id)
}

// AddJob 添加任务
func (c *Cron) AddJob(space interface{}, job Job) (EntryID, error) {
	schedule, err := c.parser.Parse(space)
	if err != nil {
		return 0, err
	}
	return c.Schedule(schedule, job), nil
}

// Schedule 添加任务到环型链表中
func (c *Cron) Schedule(schedule Schedule, job Job) EntryID {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()

	c.nextID++
	entry := &Entry{
		ID:         c.nextID,
		Schedule:   schedule,
		WrappedJob: c.chain.Then(job),
		Job:        job,
	}
	if !c.running {
		c.entries = append(c.entries, entry)
	} else {
		c.add <- entry
	}
	return entry.ID
}

// Remove 删除一个任务
func (c *Cron) Remove(id EntryID) {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.remove <- id
	} else {
		c.removeEntry(id)
	}
}

// removeEntry 从环型链表中删除一个任务
func (c *Cron) removeEntry(id EntryID) {
	// var entries []*Entry
	var entries = make([]*Entry, len(c.entries)-1)
	for index, e := range c.entries {
		if e.ID != id {
			// entries = append(entries, e) // append 的效率要低于按索引赋值
			entries[index] = e
		}
	}
	c.entries = entries
}

// now
func (c *Cron) now() time.Time {
	// In返回采用loc指定的地点和时区，但指向同一时间点的Time
	return time.Now().In(c.location)
}

// startJob 在一个新的goroutine 中去执行任务
func (c *Cron) startJob(j Job) {
	c.jobWaiter.Add(1)
	go func() {
		defer c.jobWaiter.Done()
		j.Run()
	}()
}

// entrySnapshot 返回当前环型链表的副本
func (c *Cron) entrySnapshot() []Entry {
	var entries = make([]Entry, len(c.entries))
	for i, e := range c.entries {
		entries[i] = *e
	}
	return entries
}

func (c *Cron) Stop() context.Context {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.stop <- struct{}{}
		c.running = false
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.jobWaiter.Wait()
		cancel()
	}()
	return ctx
}

func (c *Cron) run() {

	log.Println("start cron")

	// 找出每个任务下一次的激活时间
	now := c.now()
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
	}

	for {
		// 确定下一个要执行的任务
		// 通过对下一个执行时间进行排序，判断那些任务是下一次被执行的，防在队列的前面.sort是用来做排序的
		sort.Sort(byTime(c.entries))

		var timer *time.Timer
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// 如果没有,就将timer 设置在100000 * time.Hour 后触发
			timer = time.NewTimer(100000 * time.Hour)
		} else {
			// 返回下一个要执行的任务,到现在的 timer 定时器,到达指定的时间触发
			timer = time.NewTimer(c.entries[0].Next.Sub(now))
		}

		for {
			select {
			case now = <-timer.C: // 当Timer到期时，当时的时间会被发送给C
				now = now.In(c.location) // 返回当前时间的一个副本

				// 查一下下次时间比现在短的条目, 然后去执行
				for _, e := range c.entries {
					if e.Next.After(now) || e.Next.IsZero() {
						break
					}
					c.startJob(e.WrappedJob)
					// 修改链表
					e.Prev = e.Next
					e.Next = e.Schedule.Next(now)
				}

			case newEntry := <-c.add: // 往环型链表中添加
				timer.Stop()
				now = c.now()
				newEntry.Next = newEntry.Schedule.Next(now)
				c.entries = append(c.entries, newEntry)

			case replyChan := <-c.snapshot: // 复制一份新的任务链表
				replyChan <- c.entrySnapshot()
				continue

			case <-c.stop: // 是否的停止的命令
				timer.Stop()
				return

			case id := <-c.remove: // 是否是删除一个任务的命令
				timer.Stop() // 停止定时器, 要记得
				now = c.now()
				c.removeEntry(id)
			}
			break
		}
	}
}

// --------------- 定时任务相关

type FuncJob func()

func (f FuncJob) Run() { f() }

func (c *Cron) AddFunc(spec interface{}, cmd func()) (EntryID, error) {
	return c.AddJob(spec, FuncJob(cmd))
}
