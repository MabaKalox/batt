package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os/exec"
	"sync"
	"syscall"
)

type CaffeinateController struct {
	Cmd *exec.Cmd
}

var (
	controller     = &CaffeinateController{}
	controllerLock = &sync.Mutex{}
)

func StartCaffeinate() error {
	controllerLock.Lock()
	defer controllerLock.Unlock()

	if controller.Cmd != nil {
		logrus.Info("caffeinate is already running")
		return nil
	}

	cmd := exec.Command("/usr/bin/caffeinate", "-s")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start caffeinate: %w", err)
	}

	controller.Cmd = cmd
	return nil
}

func StopCaffeinate() error {
	controllerLock.Lock()
	defer controllerLock.Unlock()

	if controller.Cmd == nil || controller.Cmd.Process == nil {
		logrus.Info("caffeinate is not running")
		return nil
	}

	err := syscall.Kill(-controller.Cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to stop caffeinate: %w", err)
	}

	controller.Cmd = nil
	return nil
}
