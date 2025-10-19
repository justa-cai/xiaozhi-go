//go:build windows

package audio

/*
#cgo LDFLAGS: -lwinmm
#include <windows.h>
#include <mmsystem.h>
#include <stdlib.h>
#include <string.h>

#define BUFFER_SIZE 960  // 60ms at 16kHz

HWAVEIN hWaveIn;
WAVEHDR waveHdr;
short *buffer;

int start_recording(int sampleRate, int channels, int bufsize) {
    WAVEFORMATEX wfx;
    wfx.wFormatTag = WAVE_FORMAT_PCM;
    wfx.nChannels = channels;
    wfx.nSamplesPerSec = sampleRate;
    wfx.wBitsPerSample = 16;
    wfx.nBlockAlign = wfx.nChannels * wfx.wBitsPerSample / 8;
    wfx.nAvgBytesPerSec = wfx.nSamplesPerSec * wfx.nBlockAlign;
    wfx.cbSize = 0;

    buffer = (short*)malloc(BUFFER_SIZE * sizeof(short));
    if (!buffer) {
        return -1;
    }

    if (waveInOpen(&hWaveIn, WAVE_MAPPER, &wfx, 0, 0, CALLBACK_NULL) != MMSYSERR_NOERROR) {
        return -2;
    }

    waveHdr.lpData = (LPSTR)buffer;
    waveHdr.dwBufferLength = BUFFER_SIZE * sizeof(short);
    waveHdr.dwFlags = 0;
    waveHdr.dwLoops = 0;

    if (waveInPrepareHeader(hWaveIn, &waveHdr, sizeof(WAVEHDR)) != MMSYSERR_NOERROR) {
        return -3;
    }

    if (waveInAddBuffer(hWaveIn, &waveHdr, sizeof(WAVEHDR)) != MMSYSERR_NOERROR) {
        return -4;
    }

    if (waveInStart(hWaveIn) != MMSYSERR_NOERROR) {
        return -5;
    }

    return 0;
}

int read_pcm(int bufsize) {
    if (waveHdr.dwFlags & WHDR_DONE) {
        // 重置标志并重新准备缓冲区
        waveHdr.dwFlags &= ~WHDR_DONE;
        waveInUnprepareHeader(hWaveIn, &waveHdr, sizeof(WAVEHDR));
        waveInPrepareHeader(hWaveIn, &waveHdr, sizeof(WAVEHDR));
        waveInAddBuffer(hWaveIn, &waveHdr, sizeof(WAVEHDR));
        return BUFFER_SIZE;
    }
    return 0;
}

void stop_recording() {
    waveInStop(hWaveIn);
    waveInReset(hWaveIn);
    waveInUnprepareHeader(hWaveIn, &waveHdr, sizeof(WAVEHDR));
    free(buffer);
    waveInClose(hWaveIn);
}
*/
import "C"
import (
	"errors"
	"sync"
	"time"
	"unsafe"
)

type winRecorder struct {
	isRecording    bool
	audioCallbacks map[string]func([]byte)
	pcmCallbacks   map[string]func([]int16, int)
	stopCh         chan struct{}
	mu             sync.RWMutex
}

func newRecorder() Recorder {
	return &winRecorder{
		audioCallbacks: make(map[string]func([]byte)),
		pcmCallbacks:   make(map[string]func([]int16, int)),
	}
}

func (r *winRecorder) StartRecording(codec Encoder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isRecording {
		return errors.New("录音已在进行中")
	}
	sampleRate := 16000
	channels := 1
	framesPerBuffer := 960 // 60ms at 16kHz

	if C.start_recording(C.int(sampleRate), C.int(channels), C.int(framesPerBuffer)) != 0 {
		return errors.New("打开Windows录音设备失败")
	}
	r.isRecording = true
	r.stopCh = make(chan struct{})

	go func() {
		for {
			select {
			case <-r.stopCh:
				return
			default:
			}
			n := C.read_pcm(C.int(framesPerBuffer))
			if int(n) > 0 {
				// 取出缓冲区数据
				buf := (*[1 << 20]C.short)(unsafe.Pointer(C.buffer))[:int(n)]
				// 回调PCM数据
				r.mu.RLock()
				pcmCallbacks := make(map[string]func([]int16, int))
				for id, cb := range r.pcmCallbacks {
					pcmCallbacks[id] = cb
				}
				r.mu.RUnlock()
				
				if len(pcmCallbacks) > 0 {
					pcm := make([]int16, int(n))
					for i := 0; i < int(n); i++ {
						pcm[i] = int16(buf[i])
					}
					for _, cb := range pcmCallbacks {
						cb(pcm, int(n))
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
					b := make([]byte, int(n)*2)
					for i := 0; i < int(n); i++ {
						b[2*i] = byte(buf[i])
						b[2*i+1] = byte(buf[i] >> 8)
					}
					for _, cb := range audioCallbacks {
						cb(b)
					}
				}
				time.Sleep(10 * time.Millisecond)
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()
	return nil
}

func (r *winRecorder) StopRecording() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.isRecording {
		return nil
	}
	close(r.stopCh)
	r.isRecording = false
	C.stop_recording()
	return nil
}

func (r *winRecorder) Close() error {
	return r.StopRecording()
}

func (r *winRecorder) AddAudioDataCallback(id string, cb func([]byte)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.audioCallbacks == nil {
		r.audioCallbacks = make(map[string]func([]byte))
	}
	r.audioCallbacks[id] = cb
}

func (r *winRecorder) RemoveAudioDataCallback(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.audioCallbacks != nil {
		delete(r.audioCallbacks, id)
	}
}

func (r *winRecorder) AddPCMDataCallback(id string, cb func([]int16, int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pcmCallbacks == nil {
		r.pcmCallbacks = make(map[string]func([]int16, int))
	}
	r.pcmCallbacks[id] = cb
}

func (r *winRecorder) RemovePCMDataCallback(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pcmCallbacks != nil {
		delete(r.pcmCallbacks, id)
	}
}

func (r *winRecorder) IsRecording() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRecording
}
