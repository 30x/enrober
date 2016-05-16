package helper

import (
	"log"
	"os"
)

//LogInfo the info log level
var LogInfo = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile)

//LogWarn the warn log level
var LogWarn = log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile)

//LogError the error log level
var LogError = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile)
