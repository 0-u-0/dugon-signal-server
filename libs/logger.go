package libs

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"time"
)

var (
	Log        Logging
	logFile    *os.File
	errLogFile *os.File

	logToFile bool
)

type LogLevel = int

type Logging struct {
	Level     LogLevel
	NormalLog *log.Logger
	ErrorLog  *log.Logger
	FatalLog  *log.Logger
}

const (
	TRACE LogLevel = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

const (
	TraceColor = 34
	DebugColor = 36
	InfoColor  = 32
	WarnColor  = 33
	ErrorColor = 91
	FatalColor = 35
)

const (
	ServerTag = " [ SERVE ] |%s| - "
	TraceTag  = " [ TRACE ] |%s| - %s:%d: "
	DebugTag  = " [ DEBUG ] |%s| - %s:%d: "
	InfoTag   = " [ INFO  ] |%s| - %s:%d: "
	WarnTag   = " [ WARN  ] |%s| - %s:%d: "
	ErrorTag  = " [ ERROR ] |%s| - %s:%d: "
	FatalTag  = " [ FATAL ] |%s| - %s:%d: "
	ColorTag  = "\x1b[%dm%s\x1b[0m"
)

func GetTimeFormat() string {
	t := time.Now()
	return t.Format("2006-01-02 15:04:05")
}

func getPrefix(level LogLevel, file string, line int) string {
	color := !logToFile

	var tag string
	switch level {
	case TRACE:
		tag = fmt.Sprintf(TraceTag, GetTimeFormat(), path.Base(file), line)
		if color {
			return fmt.Sprintf(ColorTag, TraceColor, tag)
		}
	case DEBUG:
		tag = fmt.Sprintf(DebugTag, GetTimeFormat(), path.Base(file), line)
		if color {
			return fmt.Sprintf(ColorTag, DebugColor, tag)
		}
	case INFO:
		tag = fmt.Sprintf(InfoTag, GetTimeFormat(), path.Base(file), line)
		if color {
			return fmt.Sprintf(ColorTag, InfoColor, tag)
		}
	case WARN:
		tag = fmt.Sprintf(WarnTag, GetTimeFormat(), path.Base(file), line)
		if color {
			return fmt.Sprintf(ColorTag, WarnColor, tag)
		}
	case ERROR:
		tag = fmt.Sprintf(ErrorTag, GetTimeFormat(), path.Base(file), line)
		if color {
			return fmt.Sprintf(ColorTag, ErrorColor, tag)
		}
	case FATAL:
		tag = fmt.Sprintf(FatalTag, GetTimeFormat(), path.Base(file), line)
		if color {
			return fmt.Sprintf(ColorTag, FatalColor, tag)
		}
	}

	return tag
}

func (logging *Logging) Trace(v ...interface{}) {
	if logging.Level <= TRACE {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(TRACE, file, line))
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Tracef(format string, v ...interface{}) {
	if logging.Level <= TRACE {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(TRACE, file, line))
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Debug(v ...interface{}) {
	if logging.Level <= DEBUG {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(DEBUG, file, line))
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Debugf(format string, v ...interface{}) {
	if logging.Level <= DEBUG {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(DEBUG, file, line))
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Info(v ...interface{}) {
	if logging.Level <= INFO {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(INFO, file, line))
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Infof(format string, v ...interface{}) {
	if logging.Level <= INFO {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(INFO, file, line))
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Warn(v ...interface{}) {
	if logging.Level <= WARN {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(WARN, file, line))
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Warnf(format string, v ...interface{}) {
	if logging.Level <= WARN {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(getPrefix(WARN, file, line))
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Error(v ...interface{}) {
	if logging.Level <= ERROR {
		_, file, line, _ := runtime.Caller(1)

		logging.ErrorLog.SetPrefix(getPrefix(ERROR, file, line))
		logging.ErrorLog.Println(v...)
	}
}

func (logging *Logging) Errorf(format string, v ...interface{}) {
	if logging.Level <= ERROR {
		_, file, line, _ := runtime.Caller(1)

		logging.ErrorLog.SetPrefix(getPrefix(ERROR, file, line))
		logging.ErrorLog.Printf(format, v...)
	}
}

func (logging *Logging) Fatal(v ...interface{}) {
	if logging.Level <= FATAL {
		_, file, line, _ := runtime.Caller(1)

		logging.FatalLog.SetPrefix(getPrefix(FATAL, file, line))
		logging.FatalLog.Panic(v...)
	}
}

func (logging *Logging) Fatalf(format string, v ...interface{}) {
	if logging.Level <= FATAL {
		_, file, line, _ := runtime.Caller(1)

		logging.FatalLog.SetPrefix(getPrefix(FATAL, file, line))
		logging.FatalLog.Panicf(format, v...)
	}
}

func InitGlobalLog() {
	log.SetFlags(log.Lshortfile)
	log.SetPrefix(fmt.Sprintf(ServerTag, GetTimeFormat()))
}

func LoadLoggerModule(logLevel string, toFile bool, logPath, logErrorPath string) {
	logToFile = toFile

	if logToFile {
		output, err := os.OpenFile(logPath, os.O_WRONLY|os.O_SYNC|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalln(err)
			os.Exit(1)
		} else {
			logFile = output
		}

		outputErr, err := os.OpenFile(logErrorPath, os.O_WRONLY|os.O_SYNC|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalln(err)
			os.Exit(1)
		} else {
			errLogFile = outputErr
		}

	} else {
		logFile = os.Stdout
		errLogFile = os.Stderr
	}
	normalLog := log.New(logFile, "", 0)
	errorLog := log.New(errLogFile, "", 0)
	fatalLog := log.New(errLogFile, "", log.Lshortfile|log.LstdFlags)

	var level LogLevel
	switch logLevel {
	case "track":
		level = TRACE
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warn":
		level = WARN
	case "error":
		level = ERROR
	case "fatal":
		level = FATAL
	default:
		level = DEBUG
	}

	Log = Logging{level, normalLog, errorLog, fatalLog}

	log.Println("Logger loaded!")
	//	PrintModuleLoaded("Logger")

}

func ReleaseLoggerModule() {
	if logToFile {
		if logFile != nil {
			logFile.Close()
		}

		if errLogFile != nil {
			errLogFile.Close()
		}
	}
	//PrintModuleRelease("Logger")
}
