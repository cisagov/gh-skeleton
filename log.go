package main

import (
	"fmt"
	"os"
)

const (
	colorBlue   = "\x1B[34m"
	colorGreen  = "\x1B[32m"
	colorRed    = "\x1B[31m"
	colorReset  = "\x1B[0m"
	colorYellow = "\x1B[33m"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf(colorBlue+"* "+format+colorReset+"\n", args...)
}

func logOk(format string, args ...interface{}) {
	fmt.Printf(colorGreen+"✓ "+format+colorReset+"\n", args...)
}

func logWarn(format string, args ...interface{}) {
	fmt.Printf(colorYellow+"⚠︎ "+format+colorReset+"\n", args...)
}

func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorRed+"! "+format+colorReset+"\n", args...)
}
