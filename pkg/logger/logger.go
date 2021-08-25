package logger

import (
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var zapLogger *zap.Logger

func Initlog()  {

	// copy production encoder config
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05.000")

	// write log with json
	jsonEncoder := zapcore.NewJSONEncoder(cfg)
	fileWriter := newFileWriter()
	fileCore := zapcore.NewCore(jsonEncoder, fileWriter, zapcore.InfoLevel)

	// console without json
	consoleEncoder := zapcore.NewConsoleEncoder(cfg)
	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.WarnLevel)

	// create core
	core := zapcore.NewTee(fileCore, consoleCore)
	zapLogger = zap.New(core, zap.AddCaller())
	defer zapLogger.Sync()

	zap.ReplaceGlobals(zapLogger)
	zap.L().Warn("ROS start")

}

func newFileWriter()zapcore.WriteSyncer{

	lumberJackLogger := &lumberjack.Logger{
		Filename:   "./log/server.log",
		MaxSize:    1, // megabytes
		MaxBackups: 3,
		MaxAge:     28, //days
		LocalTime: true,
		Compress:   false, // disabled by default
	}

	return zapcore.AddSync(lumberJackLogger)
}
