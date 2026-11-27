// Captura o som de processos do Windows pela API de loopback por processo
// (Windows 10 build 20348 ou mais novo) e escreve PCM 48 kHz, 16 bits, estéreo
// na saída padrão. É o que permite transmitir som com fone USB, onde a captura
// do Electron falha, e sem levar junto o áudio da própria call.
//
//	concord-audio include <pid>   só o processo e os filhos dele
//	concord-audio exclude <pid>   tudo menos o processo e os filhos dele
//	concord-audio window <hwnd>   o processo dono da janela
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sampleRate    = 48000
	channels      = 2
	bytesPerFrame = 4
	wavePCM       = 1
	// 20 ms, na unidade do WASAPI (100 ns).
	bufferDuration = 200_000

	activationProcessLoopback = 1
	includeTree               = 0
	excludeTree               = 1
	vtBlob                    = 65

	streamLoopback      = 0x00020000
	streamEventCallback = 0x00040000
	streamAutoConvert   = 0x80000000
	streamSrcQuality    = 0x08000000
	bufferSilent        = 0x2

	eNoInterface = 0x80004002
)

var (
	activateAudioInterfaceAsync = windows.NewLazySystemDLL("Mmdevapi.dll").NewProc("ActivateAudioInterfaceAsync")
	getWindowThreadProcessID    = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowThreadProcessId")

	iidUnknown           = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidAgileObject       = windows.GUID{Data1: 0x94ea2b94, Data2: 0xe9cc, Data3: 0x49e0, Data4: [8]byte{0xc0, 0xff, 0xee, 0x64, 0xca, 0x8f, 0x5b, 0x90}}
	iidCompletionHandler = windows.GUID{Data1: 0x41D949AB, Data2: 0x9862, Data3: 0x444A, Data4: [8]byte{0x80, 0xF6, 0xC2, 0x61, 0x33, 0x4D, 0xA5, 0xEB}}
	iidAudioClient       = windows.GUID{Data1: 0x1CB9AD4C, Data2: 0xDBFA, Data3: 0x4c32, Data4: [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidAudioCapture      = windows.GUID{Data1: 0xC8ADBD64, Data2: 0xE71E, Data3: 0x48a0, Data4: [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
)

// Índices na vtable das interfaces COM usadas, na ordem do audioclient.h.
const (
	asyncGetActivateResult = 3

	clientInitialize     = 3
	clientStart          = 10
	clientSetEventHandle = 13
	clientGetService     = 14

	captureGetBuffer         = 3
	captureReleaseBuffer     = 4
	captureGetNextPacketSize = 5
)

type comObject struct {
	vtbl *[16]uintptr
}

func (o *comObject) call(method int, args ...uintptr) error {
	hr, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	if int32(hr) < 0 {
		return fmt.Errorf("hresult 0x%08x", uint32(hr))
	}
	return nil
}

type activationParams struct {
	activationType uint32
	targetPID      uint32
	mode           uint32
}

// PROPVARIANT com VT_BLOB, no layout de 64 bits.
type propVariant struct {
	vt   uint16
	_    [3]uint16
	size uint32
	_    uint32
	data *activationParams
}

type waveFormat struct {
	formatTag      uint16
	channels       uint16
	samplesPerSec  uint32
	avgBytesPerSec uint32
	blockAlign     uint16
	bitsPerSample  uint16
	extraSize      uint16
}

// O Windows avisa o fim da ativação num objeto COM nosso. Ele precisa ser ágil
// (IAgileObject), porque o aviso chega em outra thread.
type completionHandler struct {
	vtbl *[4]uintptr
	done windows.Handle
}

var handlerVtbl = [4]uintptr{
	windows.NewCallback(func(this *completionHandler, iid *windows.GUID, out *uintptr) uintptr {
		if *iid == iidUnknown || *iid == iidAgileObject || *iid == iidCompletionHandler {
			*out = uintptr(unsafe.Pointer(this))
			return 0
		}
		*out = 0
		return eNoInterface
	}),
	windows.NewCallback(func(this *completionHandler) uintptr { return 1 }),
	windows.NewCallback(func(this *completionHandler) uintptr { return 1 }),
	windows.NewCallback(func(this *completionHandler, op uintptr) uintptr {
		windows.SetEvent(this.done)
		return 0
	}),
}

// O Windows guarda o ponteiro do handler até o fim da ativação; a variável global
// impede o coletor de lixo de liberar o objeto antes disso.
var handler *completionHandler

func main() {
	mode, pid, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// O Electron fecha a entrada padrão ao encerrar; sem isso o processo ficaria
	// vivo para sempre enquanto nenhum som toca.
	go func() {
		io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}()
	if err := capture(mode, pid, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (mode, pid uint32, err error) {
	if len(args) != 2 {
		return 0, 0, errors.New("uso: concord-audio include|exclude|window <número>")
	}
	n, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("número inválido: %s", args[1])
	}
	switch args[0] {
	case "include":
		return includeTree, uint32(n), nil
	case "exclude":
		return excludeTree, uint32(n), nil
	case "window":
		var owner uint32
		getWindowThreadProcessID.Call(uintptr(n), uintptr(unsafe.Pointer(&owner)))
		if owner == 0 {
			return 0, 0, fmt.Errorf("janela %d não existe", n)
		}
		return includeTree, owner, nil
	}
	return 0, 0, fmt.Errorf("modo desconhecido: %s", args[0])
}

func activate(mode, pid uint32) (*comObject, error) {
	params := activationParams{activationType: activationProcessLoopback, targetPID: pid, mode: mode}
	prop := propVariant{vt: vtBlob, size: uint32(unsafe.Sizeof(params)), data: &params}
	done, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(done)
	handler = &completionHandler{vtbl: &handlerVtbl, done: done}

	device, _ := windows.UTF16PtrFromString(`VAD\Process_Loopback`)
	var op *comObject
	hr, _, _ := activateAudioInterfaceAsync.Call(
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(&iidAudioClient)),
		uintptr(unsafe.Pointer(&prop)),
		uintptr(unsafe.Pointer(handler)),
		uintptr(unsafe.Pointer(&op)),
	)
	if int32(hr) < 0 {
		return nil, fmt.Errorf("loopback por processo indisponível (Windows antigo?): hresult 0x%08x", uint32(hr))
	}
	if ev, _ := windows.WaitForSingleObject(done, 5000); ev != windows.WAIT_OBJECT_0 {
		return nil, errors.New("o Windows não respondeu à ativação do áudio")
	}
	var activateHR int32
	var client *comObject
	if err := op.call(asyncGetActivateResult, uintptr(unsafe.Pointer(&activateHR)), uintptr(unsafe.Pointer(&client))); err != nil {
		return nil, err
	}
	if activateHR < 0 {
		return nil, fmt.Errorf("ativação do áudio falhou: hresult 0x%08x", uint32(activateHR))
	}
	return client, nil
}

func capture(mode, pid uint32, out io.Writer) error {
	runtime.LockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil {
		return err
	}
	client, err := activate(mode, pid)
	if err != nil {
		return err
	}
	format := waveFormat{formatTag: wavePCM, channels: channels, samplesPerSec: sampleRate, avgBytesPerSec: sampleRate * bytesPerFrame, blockAlign: bytesPerFrame, bitsPerSample: 16}
	// Sem formato próprio no loopback por processo: o Windows converte para o nosso.
	flags := streamLoopback | streamEventCallback | streamAutoConvert | streamSrcQuality
	if err := client.call(clientInitialize, 0, uintptr(flags), bufferDuration, 0, uintptr(unsafe.Pointer(&format)), 0); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	ready, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return err
	}
	if err := client.call(clientSetEventHandle, uintptr(ready)); err != nil {
		return fmt.Errorf("set event: %w", err)
	}
	var capturer *comObject
	if err := client.call(clientGetService, uintptr(unsafe.Pointer(&iidAudioCapture)), uintptr(unsafe.Pointer(&capturer))); err != nil {
		return fmt.Errorf("capture client: %w", err)
	}
	if err := client.call(clientStart); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	w := bufio.NewWriterSize(out, 16*1024)
	silence := make([]byte, sampleRate*bytesPerFrame)
	for {
		windows.WaitForSingleObject(ready, 1000)
		for {
			var frames uint32
			if err := capturer.call(captureGetNextPacketSize, uintptr(unsafe.Pointer(&frames))); err != nil || frames == 0 {
				break
			}
			var packet *byte
			var bufFlags uint32
			if err := capturer.call(captureGetBuffer, uintptr(unsafe.Pointer(&packet)), uintptr(unsafe.Pointer(&frames)), uintptr(unsafe.Pointer(&bufFlags)), 0, 0); err != nil {
				return fmt.Errorf("get buffer: %w", err)
			}
			n := int(frames) * bytesPerFrame
			if bufFlags&bufferSilent != 0 {
				w.Write(silence[:min(n, len(silence))])
			} else {
				w.Write(unsafe.Slice(packet, n))
			}
			capturer.call(captureReleaseBuffer, uintptr(frames))
		}
		// Quem lê fechou o cano: a transmissão acabou.
		if err := w.Flush(); err != nil {
			return nil
		}
	}
}
