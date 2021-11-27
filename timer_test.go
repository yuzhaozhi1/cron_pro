package cron_pro

import (
	"fmt"
	"testing"
	"time"
)

var timerTask Timer

func TestTimer(t *testing.T) {
	timerTask = NewTimerTask()
	id, err := timerTask.AddTaskByFunc("test_01", time.Now().Add(time.Second*3), testFunc)
	fmt.Println(id, err)
	time.Sleep(time.Second * 10)
}

func testFunc(){
	defer timerTask.Remove("test_01", 1)  // 加上这个就可以实现延时任务
	printOK()
}
