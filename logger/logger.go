package logger

import (
    "io"
    "io/ioutil"
    "os"
    "path/filepath"

    "github.com/sirupsen/logrus"
    lfshook "github.com/rifflock/lfshook"
    "gopkg.in/natefinch/lumberjack.v2"
)

// Init configures logrus to write leveled logs to separate files under logDir.
func Init(logDir string) error {
    if err := os.MkdirAll(logDir, 0o755); err != nil {
        return err
    }

    // create lumberjack writers for levels
    infoWriter := &lumberjack.Logger{Filename: filepath.Join(logDir, "info.log"), MaxSize: 10, MaxBackups: 5, MaxAge: 30}
    warnWriter := &lumberjack.Logger{Filename: filepath.Join(logDir, "warn.log"), MaxSize: 10, MaxBackups: 5, MaxAge: 30}
    errorWriter := &lumberjack.Logger{Filename: filepath.Join(logDir, "error.log"), MaxSize: 10, MaxBackups: 10, MaxAge: 60}

    writerMap := lfshook.WriterMap{
        logrus.DebugLevel: infoWriter,
        logrus.InfoLevel:  infoWriter,
        logrus.WarnLevel:  warnWriter,
        logrus.ErrorLevel: errorWriter,
        logrus.FatalLevel: errorWriter,
        logrus.PanicLevel: errorWriter,
    }

    // set global formatter and level
    logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
    logrus.SetLevel(logrus.DebugLevel)
    // discard default output, we use hooks
    logrus.SetOutput(ioutil.Discard)

    hook := lfshook.NewHook(writerMap, &logrus.TextFormatter{FullTimestamp: true})
    logrus.AddHook(hook)

    // also log to stdout for convenience
    logrus.AddHook(&writerHook{out: os.Stdout})

    return nil
}

// writerHook is a tiny hook that duplicates logs to stdout.
type writerHook struct{
    out io.Writer
}

func (h *writerHook) Levels() []logrus.Level {
    return logrus.AllLevels
}

func (h *writerHook) Fire(entry *logrus.Entry) error {
    line, err := entry.String()
    if err != nil {
        return err
    }
    _, _ = h.out.Write([]byte(line))
    return nil
}
