package cron_pro

import (
	"fmt"
	"testing"
	"time"
)

func TestCron(t *testing.T) {
	cron := NewCron()
	now := time.Now().Add(time.Second * 5)
	fmt.Println(now)
	// id, err := cron.AddFunc("@every 1s", printOK)
	// if err != nil {
	// 	fmt.Println("create cron task failed, err: ", err)
	// 	return
	// }
	_, err := cron.AddFunc(now, PrintHH)
	if err != nil {
		fmt.Println("create cron task failed, err: ", err)
		return
	}

	cron.Start()
	time.Sleep(time.Second *8)
}

func printOK () {
	fmt.Println("ok!")
}
func PrintHH(){
	fmt.Println("hhh")
}
