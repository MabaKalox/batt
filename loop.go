package main

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	maintainedChargingInProgress atomic.Bool
	maintainLoopLock             = &sync.Mutex{}
	enableMaintainLoop           atomic.Bool
)

const loopInterval = time.Duration(10) * time.Second

func init() {
	enableMaintainLoop.Store(true)
}

// infiniteLoop runs forever and maintains the battery charge,
// which is called by the daemon.
func infiniteLoop() {
	for {
		if enableMaintainLoop.Load() {
			maintainLoop()
		}
		time.Sleep(loopInterval)
	}
}

// maintainLoop maintains the battery charge. It has the logic to
// prevent parallel runs. So if one maintain loop is already running,
// the next one will need to wait until the first one finishes.
func maintainLoop() bool {
	maintainLoopLock.Lock()
	defer maintainLoopLock.Unlock()

	upper := config.Limit
	delta := config.LowerLimitDelta
	lower := upper - delta
	maintain := upper < 100

	isChargingEnabled, err := smcConn.IsChargingEnabled()
	if err != nil {
		logrus.Errorf("IsChargingEnabled failed: %v", err)
		return false
	}

	// If maintain is disabled, we don't care about the battery charge, enable charging anyway.
	if !maintain {
		logrus.Debug("limit set to 100%, maintain loop disabled")
		if !isChargingEnabled {
			logrus.Debug("charging disabled, enabling")
			err = smcConn.EnableCharging()
			if err != nil {
				logrus.Errorf("EnableCharging failed: %v", err)
				return false
			}
		}
		maintainedChargingInProgress.Store(true)
		return true
	}

	batteryCharge, err := smcConn.GetBatteryCharge()
	if err != nil {
		logrus.Errorf("GetBatteryCharge failed: %v", err)
		return false
	}

	isPluggedIn, err := smcConn.IsPluggedIn()
	if err != nil {
		logrus.Errorf("IsPluggedIn failed: %v", err)
		return false
	}

	maintainedChargingInProgress.Store(isChargingEnabled && isPluggedIn)

	printStatus(batteryCharge, lower, upper, isChargingEnabled, isPluggedIn, maintainedChargingInProgress.Load())

	if batteryCharge < lower && !isChargingEnabled {
		logrus.WithFields(logrus.Fields{
			"batteryCharge": batteryCharge,
			"lower":         lower,
			"upper":         upper,
			"delta":         delta,
		}).Infof("Battery charge is below lower limit, enabling charging")

		err = StartCaffeinate()
		if err != nil {
			logrus.Errorf("StartCaffeinate before EnableCharging failed: %v", err)
			return false
		}

		err = smcConn.EnableCharging()
		if err != nil {
			logrus.Errorf("EnableCharging failed: %v", err)
			return false
		}
		isChargingEnabled = true
		maintainedChargingInProgress.Store(true)
	}

	if batteryCharge >= upper && isChargingEnabled {
		logrus.WithFields(logrus.Fields{
			"batteryCharge": batteryCharge,
			"lower":         lower,
			"upper":         upper,
			"delta":         delta,
		}).Infof("Battery charge is above upper limit, disabling charging")

		err = StopCaffeinate()
		if err != nil {
			logrus.Errorf("StopCaffeinate before DisableCharging failed: %v", err)
			return false
		}

		err = smcConn.DisableCharging()
		if err != nil {
			logrus.Errorf("DisableCharging failed: %v", err)
			return false
		}
		isChargingEnabled = false
		maintainedChargingInProgress.Store(false)
	}

	if config.ControlMagSafeLED {
		updateMagSafeLed(isChargingEnabled)
	}

	// batteryCharge >= upper - delta && batteryCharge < upper
	// do nothing, keep as-is

	return true
}

func updateMagSafeLed(isChargingEnabled bool) {
	err := smcConn.SetMagSafeCharging(isChargingEnabled)
	if err != nil {
		logrus.Errorf("SetMagSafeCharging failed: %v", err)
	}
}

var lastPrintTime time.Time

type loopStatus struct {
	batteryCharge                int
	lower                        int
	upper                        int
	isChargingEnabled            bool
	isPluggedIn                  bool
	maintainedChargingInProgress bool
}

var lastStatus loopStatus

func printStatus(
	batteryCharge int,
	lower int,
	upper int,
	isChargingEnabled bool,
	isPluggedIn bool,
	maintainedChargingInProgress bool,
) {
	currentStatus := loopStatus{
		batteryCharge:                batteryCharge,
		lower:                        lower,
		upper:                        upper,
		isChargingEnabled:            isChargingEnabled,
		isPluggedIn:                  isPluggedIn,
		maintainedChargingInProgress: maintainedChargingInProgress,
	}

	fields := logrus.Fields{
		"batteryCharge":                batteryCharge,
		"lower":                        lower,
		"upper":                        upper,
		"chargingEnabled":              isChargingEnabled,
		"isPluggedIn":                  isPluggedIn,
		"maintainedChargingInProgress": maintainedChargingInProgress,
	}

	defer func() { lastPrintTime = time.Now() }()

	// Skip printing if the last print was less than loopInterval+1 seconds ago and everything is the same.
	if time.Since(lastPrintTime) < loopInterval+time.Second && reflect.DeepEqual(lastStatus, currentStatus) {
		logrus.WithFields(fields).Trace("maintain loop status")
		return
	}

	logrus.WithFields(fields).Debug("maintain loop status")

	lastStatus = currentStatus
}
