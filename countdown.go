package main

import "time"

type CountDownHandle struct {
	ch chan struct{}
}

func NewCountDown(action func(), seconds int) *CountDownHandle {
	ch := make(chan struct{})

	go func() {
		for range ch {
			// Using Ticker instead of time.After or time.Sleep because when the computer sleeps, we
			// actually want the countDown to prolong as well.
			const tl = 250 // ms
			t := time.NewTicker(time.Duration(tl) * time.Millisecond)
			ticksWanted := seconds * 1000 / tl
			ticksElapsed := 0

			for range t.C {
				// Check if we got new start request, then reset timer
				select {
				case <-ch:
					ticksElapsed = 0
				default:
				}

				ticksElapsed++
				if ticksElapsed > ticksWanted {
					break
				}
			}
			t.Stop()

			action()
		}
	}()

	return &CountDownHandle{ch}
}
