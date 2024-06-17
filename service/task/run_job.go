package task

import (
	"fmt"
	"github.com/gorhill/cronexpr"
	"metaid-market-service/major"
	"time"
)

func RunJob() {
	executeCronJob()
}

func executeCronJob() {
	var (
		newSchedule map[string]*cronJob = make(map[string]*cronJob)
		now                             = time.Now()

		newCronJobForCheckRecordConfirm *cronJob
		exprForCheckRecordConfirm       = cronexpr.MustParse("0 */10 * * * * *") //per 10m

		newCronJobForCheckAssetConfirm *cronJob
		exprForCheckAssetConfirm       = cronexpr.MustParse("0 */10 * * * * *") //per 10m

	)
	newCronJobForCheckRecordConfirm = &cronJob{exprForCheckRecordConfirm, exprForCheckRecordConfirm.Next(now), jobForCheckRecordConfirm}
	newSchedule["[Job-Record-Confirm]"] = newCronJobForCheckRecordConfirm

	newCronJobForCheckAssetConfirm = &cronJob{exprForCheckAssetConfirm, exprForCheckAssetConfirm.Next(now), JobForCheckAssetConfirm}
	newSchedule["[Job-Asset-Confirm]"] = newCronJobForCheckAssetConfirm

	SetCronJobList(newSchedule)

	major.Println(fmt.Sprintf("Executing Schedule \n"))
	runCronJob()
}
