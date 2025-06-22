package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func setupLogger() error {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %v", err)
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return nil
}

func main() {
	fmt.Println("Maba modified batt, v2.0!")
	cmd := NewCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
