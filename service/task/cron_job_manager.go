package task

import (
	"fmt"
	"github.com/gorhill/cronexpr"
	"metaid-market-service/major"
	"time"
)

// task
type cronJob struct {
	expr     *cronexpr.Expression
	nextTime time.Time // expr.Next(now)
	taskFunc func()
}

var (
	schedule map[string]*cronJob = make(map[string]*cronJob)
)

func SetCronJobList(newSchedule map[string]*cronJob) {
	if newSchedule == nil || len(newSchedule) == 0 {
		return
	}
	if schedule == nil {
		return
	}
	for jobName, cronJob := range newSchedule {
		if cronJob == nil || cronJob.expr == nil || cronJob.taskFunc == nil {
			continue
		}
		schedule[jobName] = cronJob
	}
}

func runCronJob() {
	go func() {
		var (
			jobName string
			cronJob *cronJob
			now     time.Time
		)
		for {
			now = time.Now()
			for jobName, cronJob = range schedule {
				if cronJob.nextTime.Before(now) || cronJob.nextTime.Equal(now) {

					//logger.Logger.Infof("Executing Job:[%s] \n", jobName)
					major.Println(fmt.Sprintf("Executing Job:[%s] \n", jobName))
					cronJob.taskFunc()
					//logger.Logger.Infof("Finished Job:[%s] \n", jobName)
					major.Println(fmt.Sprintf("Finished Job:[%s] \n", jobName))

					cronJob.nextTime = cronJob.expr.Next(now)
					fmt.Println(jobName, "Next execution time:", cronJob.nextTime)
				}
			}

			select {
			case <-time.NewTimer(10 * 1000 * time.Millisecond).C: // 将在100毫秒可读，返回
			}
		}
	}()
}
