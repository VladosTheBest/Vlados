package crons

// SystemRevertOrderChan godoc
var SystemRevertOrderChan = make(chan bool, 10)

// CronSystemRevertFrozenOrder godoc
func CronSystemRevertFrozenOrder() {
	SystemRevertOrderChan <- true
}
