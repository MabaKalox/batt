package main

/*
#cgo LDFLAGS: -framework IOKit
#include "hook.h"
*/
import "C"

import (
	"fmt"
	"github.com/sirupsen/logrus"
)

const postSleepLoopDelaySeconds = 30

var countDownEnableMaintainLoop *CountDownHandle

func init() {
	countDownEnableMaintainLoop = NewCountDown(func() { enableMaintainLoop.Store(true) }, postSleepLoopDelaySeconds)
}

//export canSystemSleepCallback
func canSystemSleepCallback() {
	/* Idle sleep is about to kick in. This message will not be sent for forced sleep.
	   Applications have a chance to prevent sleep by calling IOCancelPowerChange.
	   Most applications should not prevent idle sleep.

	   Power Management waits up to 30 seconds for you to either allow or deny idle
	   sleep. If you don't acknowledge this power change by calling either
	   IOAllowPowerChange or IOCancelPowerChange, the system will wait 30
	   seconds then go to sleep.
	*/
	logrus.Debugln("received kIOMessageCanSystemSleep notification, idle sleep is about to kick in")

	// We won't allow idle sleep if the system has just waked up,
	// because there may still be a maintain loop waiting (see the wg.Wait() in loop.go).
	// So decisions may not be made yet. We need to wait.
	// Actually, we wait the larger of preSleepLoopDelaySeconds and postSleepLoopDelaySeconds. This is not implemented yet.
	//if timeAfterWokenUp := time.Since(lastWakeTime); timeAfterWokenUp < time.Duration(preSleepLoopDelaySeconds)*time.Second {
	//	logrus.Debugf("system has just waked up (%fs ago), deny idle sleep", timeAfterWokenUp.Seconds())
	//	C.CancelPowerChange()
	//	return
	//}

	C.AllowPowerChange()
}

//export systemWillSleepCallback
func systemWillSleepCallback() {
	/* The system WILL go to sleep. If you do not call IOAllowPowerChange or
	   IOCancelPowerChange to acknowledge this message, sleep will be
	   delayed by 30 seconds.

	   NOTE: If you call IOCancelPowerChange to deny sleep it returns
	   kIOReturnSuccess, however the system WILL still go to sleep.
	*/
	logrus.Debugln("received kIOMessageSystemWillSleep notification, system will go to sleep")

	enableMaintainLoop.Store(false)

	C.AllowPowerChange()
}

//export systemWillPowerOnCallback
func systemWillPowerOnCallback() {
	// System has started the wake-up process...
}

//export systemHasPoweredOnCallback
func systemHasPoweredOnCallback() {
	// System has finished waking up...
	logrus.Debugln("received kIOMessageSystemHasPoweredOn notification, system has finished waking up")

	// Trigger update, to check if we need to charge (will prevent sleep) and update MagSafe
	maintainLoop()

	countDownEnableMaintainLoop.ch <- struct{}{}
}

func listenNotifications() error {
	logrus.Info("registered and listening system sleep notifications")
	if int(C.ListenNotifications()) != 0 {
		return fmt.Errorf("IORegisterForSystemPower failed")
	}
	return nil
}

func stopListeningNotifications() {
	C.StopListeningNotifications()
	logrus.Info("stopped listening system sleep notifications")
}
