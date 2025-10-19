//go:build linux

package audio

/*
#cgo pkg-config: libpulse-simple
#include <pulse/simple.h>
#include <pulse/error.h>
#include <stdlib.h>

typedef struct pa_simple pa_simple;

static pa_simple* open_pulse_capture(unsigned int sampleRate, int channels, int* error) {
    pa_sample_spec ss;
    ss.format = PA_SAMPLE_S16LE;
    ss.rate = sampleRate;
    ss.channels = channels;
    return pa_simple_new(NULL, "xiaozhi-go", PA_STREAM_RECORD, NULL, "record", &ss, NULL, NULL, error);
}
static int read_pulse(pa_simple* s, void* buf, int bytes, int* error) {
    return pa_simple_read(s, buf, bytes, error);
}
static void close_pulse(pa_simple* s) {
    if (s) pa_simple_free(s);
}
*/
import "C"
import (
	"errors"
	"sync"
	"unsafe"
)

type linuxRecorder struct {
	isRecording      bool
	audioCallbacks   map[string]func([]byte)
	pcmCallbacks     map[string]func([]int16, int)
	stopCh           chan struct{}
	mu               sync.RWMutex
	handle           *C.pa_simple
	wg               sync.WaitGroup
}

func newRecorder() Recorder {
	return &linuxRecorder{
		audioCallbacks: make(map[string]func([]byte)),
		pcmCallbacks:   make(map[string]func([]int16, int)),
	}
}

func (r *linuxRecorder) StartRecording(codec Encoder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isRecording {
		return errors.New("录音已在进行中")
	}
	var errorCode C.int
	sampleRate := C.uint(16000)
	channels := C.int(1)
	framesPerBuffer := 960 // 60ms at 16kHz
	bytesPerFrame := int(channels) * 2
	bufSize := framesPerBuffer * bytesPerFrame

	h := C.open_pulse_capture(sampleRate, channels, &errorCode)
	if h == nil {
		return errors.New("打开PulseAudio录音设备失败")
	}
	r.handle = h
	r.isRecording = true
	r.stopCh = make(chan struct{})
	r.wg.Add(1)

	go func() {
		defer r.wg.Done()
		buf := make([]int16, framesPerBuffer*int(channels))
		byteBuf := (*[1 << 20]byte)(unsafe.Pointer(&buf[0]))[:bufSize]
		for {
			select {
			case <-r.stopCh:
				return
			default:
			}
			if C.read_pulse(r.handle, unsafe.Pointer(&buf[0]), C.int(bufSize), &errorCode) != 0 {
				continue // 采集失败，跳过
			}
			// 回调PCM数据
			r.mu.RLock()
			pcmCallbacks := make(map[string]func([]int16, int))
			for id, cb := range r.pcmCallbacks {
				pcmCallbacks[id] = cb
			}
			r.mu.RUnlock()
			
			if len(pcmCallbacks) > 0 {
				pcmCopy := make([]int16, framesPerBuffer*int(channels))
				copy(pcmCopy, buf[:framesPerBuffer*int(channels)])
				for _, cb := range pcmCallbacks {
					cb(pcmCopy, framesPerBuffer*int(channels))
				}
			}
			
			// 回调原始字节数据
			r.mu.RLock()
			audioCallbacks := make(map[string]func([]byte))
			for id, cb := range r.audioCallbacks {
				audioCallbacks[id] = cb
			}
			r.mu.RUnlock()
			
			if len(audioCallbacks) > 0 {
				dataCopy := make([]byte, bufSize)
				copy(dataCopy, byteBuf[:bufSize])
				for _, cb := range audioCallbacks {
					cb(dataCopy)
				}
			}
		}
	}()
	return nil
}

func (r *linuxRecorder) StopRecording() error {
	r.mu.Lock()
	if !r.isRecording {
		r.mu.Unlock()
		return nil
	}
	close(r.stopCh)
	r.isRecording = false
	handle := r.handle
	r.handle = nil
	r.mu.Unlock()

	// 等待录音goroutine退出后再释放handle
	r.wg.Wait()
	if handle != nil {
		C.close_pulse(handle)
	}
	return nil
}

func (r *linuxRecorder) Close() error {
	return r.StopRecording()
}

func (r *linuxRecorder) AddAudioDataCallback(id string, cb func([]byte)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.audioCallbacks == nil {
		r.audioCallbacks = make(map[string]func([]byte))
	}
	r.audioCallbacks[id] = cb
}

func (r *linuxRecorder) RemoveAudioDataCallback(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.audioCallbacks != nil {
		delete(r.audioCallbacks, id)
	}
}

func (r *linuxRecorder) AddPCMDataCallback(id string, cb func([]int16, int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pcmCallbacks == nil {
		r.pcmCallbacks = make(map[string]func([]int16, int))
	}
	r.pcmCallbacks[id] = cb
}

func (r *linuxRecorder) RemovePCMDataCallback(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pcmCallbacks != nil {
		delete(r.pcmCallbacks, id)
	}
}

func (r *linuxRecorder) IsRecording() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRecording
}
