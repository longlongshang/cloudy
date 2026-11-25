//go:build darwin

package netease

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	secCSSigningInformation = 1 << 1
	secCodeSignatureAdhoc   = 0x0002
	cfNumberSInt32Type      = 3
)

type secCodeAPI struct {
	copySelf        func(flags uint32, self *uintptr) int32
	copyStaticCode  func(code uintptr, flags uint32, staticCode *uintptr) int32
	createWithPath  func(path uintptr, flags uint32, staticCode *uintptr) int32
	copySigningInfo func(code uintptr, flags uint32, info *uintptr) int32
	urlFromPath     func(alloc uintptr, buf *byte, bufLen int64, isDir uint8) uintptr
	dictGetValue    func(dict, key uintptr) uintptr
	numberGetValue  func(num uintptr, typ int64, valuePtr unsafe.Pointer) uint8
	arrayGetCount   func(arr uintptr) int64
	release         func(ref uintptr)

	kInfoFlags        uintptr
	kInfoCertificates uintptr
}

var loadSecCodeAPI = sync.OnceValue(func() *secCodeAPI {
	sec, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return nil
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return nil
	}
	kFlags := cfConst(sec, "kSecCodeInfoFlags")
	kCerts := cfConst(sec, "kSecCodeInfoCertificates")
	if kFlags == 0 || kCerts == 0 {
		return nil
	}
	a := &secCodeAPI{
		kInfoFlags:        kFlags,
		kInfoCertificates: kCerts,
	}
	purego.RegisterLibFunc(&a.copySelf, sec, "SecCodeCopySelf")
	purego.RegisterLibFunc(&a.copyStaticCode, sec, "SecCodeCopyStaticCode")
	purego.RegisterLibFunc(&a.createWithPath, sec, "SecStaticCodeCreateWithPath")
	purego.RegisterLibFunc(&a.copySigningInfo, sec, "SecCodeCopySigningInformation")
	purego.RegisterLibFunc(&a.urlFromPath, cf, "CFURLCreateFromFileSystemRepresentation")
	purego.RegisterLibFunc(&a.dictGetValue, cf, "CFDictionaryGetValue")
	purego.RegisterLibFunc(&a.numberGetValue, cf, "CFNumberGetValue")
	purego.RegisterLibFunc(&a.arrayGetCount, cf, "CFArrayGetCount")
	purego.RegisterLibFunc(&a.release, cf, "CFRelease")
	return a
})

func cfConst(lib uintptr, name string) uintptr {
	p, err := purego.Dlsym(lib, name)
	if err != nil || p == 0 {
		return 0
	}
	return **(**uintptr)(unsafe.Pointer(&p))
}

// preferSessionFile reports whether the running macOS binary lacks a
// certificate-backed code signature (unsigned or ad-hoc signed). Such builds
// receive a new cdhash on every update, so storing the login cookie in the
// login Keychain would prompt for a password or lose access after updating.
var preferSessionFile = sync.OnceValue(func() bool {
	api := loadSecCodeAPI()
	if api == nil {
		return false
	}
	var self uintptr
	if api.copySelf(0, &self) != 0 || self == 0 {
		return true
	}
	defer api.release(self)
	var staticCode uintptr
	if api.copyStaticCode(self, 0, &staticCode) != 0 || staticCode == 0 {
		return true
	}
	defer api.release(staticCode)
	return !api.hasCertificateSignature(staticCode)
})

func (a *secCodeAPI) hasCertificateSignature(staticCode uintptr) bool {
	var info uintptr
	if a.copySigningInfo(staticCode, secCSSigningInformation, &info) != 0 || info == 0 {
		return false
	}
	defer a.release(info)
	if flagsNum := a.dictGetValue(info, a.kInfoFlags); flagsNum != 0 {
		var flags uint32
		if a.numberGetValue(flagsNum, cfNumberSInt32Type, unsafe.Pointer(&flags)) != 0 && flags&secCodeSignatureAdhoc != 0 {
			return false
		}
	}
	certs := a.dictGetValue(info, a.kInfoCertificates)
	return certs != 0 && a.arrayGetCount(certs) > 0
}

func pathHasCertificateSignature(path string) bool {
	api := loadSecCodeAPI()
	if api == nil || len(path) == 0 {
		return false
	}
	b := []byte(path)
	url := api.urlFromPath(0, &b[0], int64(len(b)), 0)
	if url == 0 {
		return false
	}
	defer api.release(url)
	var staticCode uintptr
	if api.createWithPath(url, 0, &staticCode) != 0 || staticCode == 0 {
		return false
	}
	defer api.release(staticCode)
	return api.hasCertificateSignature(staticCode)
}
