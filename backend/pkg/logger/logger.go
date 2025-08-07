package logger

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func Init(level, format string) error {
	log = logrus.New()
	
	// 设置日志级别
	switch level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
	
	// 设置日志格式
	switch format {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	default:
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
	
	log.SetOutput(os.Stdout)
	
	return nil
}

func Debug(args ...interface{}) {
	if log != nil {
		log.Debug(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if log != nil {
		log.Debugf(format, args...)
	}
}

func Info(args ...interface{}) {
	if log != nil {
		log.Info(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if log != nil {
		log.Infof(format, args...)
	}
}

func Warn(args ...interface{}) {
	if log != nil {
		log.Warn(args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if log != nil {
		log.Warnf(format, args...)
	}
}

func Error(args ...interface{}) {
	if log != nil {
		log.Error(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if log != nil {
		log.Errorf(format, args...)
	} else {
		fmt.Printf("ERROR: "+format+"\n", args...)
	}
}

func Fatal(args ...interface{}) {
	if log != nil {
		log.Fatal(args...)
	} else {
		fmt.Print("FATAL: ")
		fmt.Println(args...)
		os.Exit(1)
	}
}

func Fatalf(format string, args ...interface{}) {
	if log != nil {
		log.Fatalf(format, args...)
	} else {
		fmt.Printf("FATAL: "+format+"\n", args...)
		os.Exit(1)
	}
}