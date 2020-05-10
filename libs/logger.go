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
	VERBOSE LogLevel = iota
	DEBUG
	INFO
	WARNING
	ERROR
	FATAL
)

const (
	ServerTag  = " [ SERV ] |%s| - "
	VerboseTag = " [ VERB ] "
	DebugTag   = " [ DEBU ] "
	InfoTag    = " [ INFO ] |%s| - %s:%d: "
	WarnTag    = " [ WARN ] |%s| - %s:%d: "
	ErrorTag   = " [ ERRO ] |%s| - %s:%d: "
	FatalTag   = " [ FATA ] |%s| - %s:%d: "
)

func GetTimeFormat() string {
	t := time.Now()
	return t.Format("2006-01-02 15:04:05")
}

func (logging *Logging) Verbose(v ...interface{}) {
	if logging.Level <= VERBOSE {
		logging.NormalLog.SetPrefix(VerboseTag)
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Verbosef(format string, v ...interface{}) {
	if logging.Level <= VERBOSE {
		logging.NormalLog.SetPrefix(VerboseTag)
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Debug(v ...interface{}) {
	if logging.Level <= DEBUG {
		logging.NormalLog.SetPrefix(DebugTag)
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Debugf(format string, v ...interface{}) {
	if logging.Level <= DEBUG {
		logging.NormalLog.SetPrefix(DebugTag)
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Info(v ...interface{}) {
	if logging.Level <= INFO {
		_, file, line, _ := runtime.Caller(1)

		logging.NormalLog.SetPrefix(fmt.Sprintf(InfoTag, GetTimeFormat(), path.Base(file), line))
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Infof(format string, v ...interface{}) {
	if logging.Level <= INFO {
		_, file, line, _ := runtime.Caller(1)
		logging.NormalLog.SetPrefix(fmt.Sprintf(InfoTag, GetTimeFormat(), path.Base(file), line))
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Warning(v ...interface{}) {
	if logging.Level <= WARNING {
		_, file, line, _ := runtime.Caller(1)
		logging.NormalLog.SetPrefix(fmt.Sprintf(WarnTag, GetTimeFormat(), path.Base(file), line))
		logging.NormalLog.Println(v...)
	}
}

func (logging *Logging) Warningf(format string, v ...interface{}) {
	if logging.Level <= WARNING {
		_, file, line, _ := runtime.Caller(1)
		logging.NormalLog.SetPrefix(fmt.Sprintf(WarnTag, GetTimeFormat(), path.Base(file), line))
		logging.NormalLog.Printf(format, v...)
	}
}

func (logging *Logging) Error(v ...interface{}) {
	if logging.Level <= ERROR {
		_, file, line, _ := runtime.Caller(1)
		logging.ErrorLog.SetPrefix(fmt.Sprintf(ErrorTag, GetTimeFormat(), path.Base(file), line))
		logging.ErrorLog.Println(v...)
	}
}

func (logging *Logging) Errorf(format string, v ...interface{}) {
	if logging.Level <= ERROR {
		_, file, line, _ := runtime.Caller(1)
		logging.ErrorLog.SetPrefix(fmt.Sprintf(ErrorTag, GetTimeFormat(), path.Base(file), line))
		logging.ErrorLog.Printf(format, v...)
	}
}

func (logging *Logging) Fatal(v ...interface{}) {
	if logging.Level <= FATAL {
		_, file, line, _ := runtime.Caller(1)
		logging.FatalLog.SetPrefix(fmt.Sprintf(FatalTag, GetTimeFormat(), path.Base(file), line))
		logging.FatalLog.Panic(v...)
	}
}

func (logging *Logging) Fatalf(format string, v ...interface{}) {
	if logging.Level <= FATAL {
		_, file, line, _ := runtime.Caller(1)
		logging.FatalLog.SetPrefix(fmt.Sprintf(FatalTag, GetTimeFormat(), path.Base(file), line))
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
	case "verbose":
		level = VERBOSE
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warning":
		level = WARNING
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
