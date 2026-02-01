// Gophermart is a loyalty points accumulation system.
package main

import (
	"gophermart/internal/app"
	"gophermart/internal/log"
)

func main() {
	logger := log.NewLogger()
	defer logger.Sync()

	application, err := app.New(logger)
	if err != nil {
		logger.Fatal("failed to initialize application", "error", err)
	}

	if err := application.Run(); err != nil {
		logger.Fatal("application failed to run", "error", err)
	}
}
