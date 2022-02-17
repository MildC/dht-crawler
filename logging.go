package main

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	colorMsgEncoding = "console-with-color"
)

type colorMsgEncoder struct {
	zapcore.Encoder
}

func (enc *colorMsgEncoder) Clone() zapcore.Encoder {
	return &colorMsgEncoder{enc.Encoder.Clone()}
}

func (enc *colorMsgEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	var colorFormatString string
	switch entry.Level {
	case zapcore.DebugLevel:
		colorFormatString = "\x1b[38;2;127;132;142m%s\x1b[0m"
	case zapcore.WarnLevel:
		colorFormatString = "\x1b[38;2;229;192;122m%s\x1b[0m"
	case zapcore.ErrorLevel:
		colorFormatString = "\x1b[38;2;224;107;106m%s\x1b[0m"
	default:
		colorFormatString = "\x1b[38;2;255;255;255m%s\x1b[0m"
	}
	// ignore all fields - passing a nil slice onwards instead
	entry.Message = fmt.Sprintf(colorFormatString, entry.Message)
	return enc.Encoder.EncodeEntry(entry, fields)
}

func init() {
	err := zap.RegisterEncoder(colorMsgEncoding, func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return &colorMsgEncoder{zapcore.NewConsoleEncoder(config)}, nil
	})

	if err != nil {
		panic(err)
	}
}

func NewConsoleLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Encoding = colorMsgEncoding
	config.EncoderConfig.LevelKey = zapcore.OmitKey
	config.EncoderConfig.CallerKey = zapcore.OmitKey
	logger, _ := config.Build()
	return logger
}

var (
	_ zapcore.Encoder = &colorMsgEncoder{}
)
