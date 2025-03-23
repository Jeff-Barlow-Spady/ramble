//go:build cgo && whisper_cgo
// +build cgo,whisper_cgo

package transcription

/*
#cgo CFLAGS: -I${SRCDIR}/../../whisper.cpp/include
#cgo LDFLAGS: -L${SRCDIR}/../../whisper.cpp/build -lwhisper

#include <stdlib.h>
#include <whisper.h>
#include <stdio.h>

// This file contains C callback implementations for the whisper.cpp library

// Forward declaration for the Go callback function
extern void goWhisperLogCallback(int level, char* text, void* user_data);

// C wrapper for the whisper log callback
void whisperLogCallback(enum whisper_log_level level, const char * text, void * user_data) {
    goWhisperLogCallback((int)level, (char*)text, user_data);
}
*/
import "C"
import (
	"unsafe"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// Export the Go callback function for C to call
//
//export goWhisperLogCallback
func goWhisperLogCallback(level C.int, text *C.char, userData unsafe.Pointer) {
	// Convert C string to Go string
	goText := C.GoString(text)

	// Log the message based on the level
	switch level {
	case 0: // WHISPER_LOG_LEVEL_ERROR
		logger.Error(logger.CategoryTranscription, "Whisper error: %s", goText)
	case 1: // WHISPER_LOG_LEVEL_WARN
		logger.Warning(logger.CategoryTranscription, "Whisper warning: %s", goText)
	case 2: // WHISPER_LOG_LEVEL_INFO
		logger.Info(logger.CategoryTranscription, "Whisper info: %s", goText)
	default:
		logger.Debug(logger.CategoryTranscription, "Whisper debug: %s", goText)
	}
}
