//go:build darwin

package audio

/*
#cgo LDFLAGS: -framework CoreAudio -framework AudioUnit -framework AudioToolbox
#include <CoreAudio/CoreAudio.h>
#include <AudioUnit/AudioUnit.h>
#include <AudioToolbox/AudioToolbox.h>
#include <stdlib.h>

// 这里只做骨架，建议后续用go-mac/coreaudio或cgo补全
*/
import "C"
import (
	"errors"
	"sync"
)

type darwinRecorder struct {
	isRecording    bool
	audioCallbacks map[string]func([]byte)
	pcmCallbacks   map[string]func([]int16, int)
	stopCh         chan struct{}
	mu             sync.RWMutex
}

func newRecorder() Recorder {
	return &darwinRecorder{
		audioCallbacks: make(map[string]func([]byte)),
		pcmCallbacks:   make(map[string]func([]int16, int)),
	}
}

func (r *darwinRecorder) StartRecording(codec Encoder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isRecording {
		return errors.New("录音已在进行中")
	}
	// TODO: 这里需要用CoreAudio API实现音频采集
	r.isRecording = true
	r.stopCh = make(chan struct{})
	// 伪实现：直接返回未实现
	go func() {
		// 你可以在这里实现CoreAudio采集并回调
	}()
	return errors.New("macOS录音功能未实现")
}

func (r *darwinRecorder) StopRecording() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.isRecording {
		return nil
	}
	close(r.stopCh)
	r.isRecording = false
	return nil
}
func (r *darwinRecorder) Close() error {
	return r.StopRecording()
}
func (r *darwinRecorder) AddAudioDataCallback(id string, cb func([]byte)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.audioCallbacks == nil {
		r.audioCallbacks = make(map[string]func([]byte))
	}
	r.audioCallbacks[id] = cb
}

func (r *darwinRecorder) RemoveAudioDataCallback(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.audioCallbacks != nil {
		delete(r.audioCallbacks, id)
	}
}

func (r *darwinRecorder) AddPCMDataCallback(id string, cb func([]int16, int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pcmCallbacks == nil {
		r.pcmCallbacks = make(map[string]func([]int16, int))
	}
	r.pcmCallbacks[id] = cb
}

func (r *darwinRecorder) RemovePCMDataCallback(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pcmCallbacks != nil {
		delete(r.pcmCallbacks, id)
	}
}
func (r *darwinRecorder) IsRecording() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRecording
}
