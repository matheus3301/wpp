package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a zap logger that writes JSON to the given log file path
// and also writes to stderr. Session name and PID are included as initial fields.
func New(logPath, sessionName string) (*zap.Logger, error) {
	if err := os.MkdirAll(logPath[:len(logPath)-len("/wppd.log")], 0700); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	jsonEncoder := zapcore.NewJSONEncoder(encoderCfg)
	consoleEncoder := zapcore.NewConsoleEncoder(encoderCfg)

	fileCore := zapcore.NewCore(jsonEncoder, zapcore.AddSync(file), zapcore.InfoLevel)
	stderrCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), zapcore.InfoLevel)

	core := zapcore.NewTee(fileCore, stderrCore)

	logger := zap.New(core,
		zap.Fields(
			zap.String("session", sessionName),
			zap.Int("pid", os.Getpid()),
		),
	)

	return logger, nil
}
