package log

import "log"

func Infof(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func Errorf(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}
