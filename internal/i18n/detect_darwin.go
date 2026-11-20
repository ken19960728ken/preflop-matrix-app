//go:build darwin && cgo

package i18n

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

static char *firstPreferredLanguage(void) {
	CFArrayRef langs = CFLocaleCopyPreferredLanguages();
	if (langs == NULL) {
		return NULL;
	}
	char *out = NULL;
	if (CFArrayGetCount(langs) > 0) {
		CFStringRef s = (CFStringRef)CFArrayGetValueAtIndex(langs, 0);
		CFIndex n = CFStringGetMaximumSizeForEncoding(CFStringGetLength(s), kCFStringEncodingUTF8) + 1;
		out = malloc(n);
		if (out != NULL && !CFStringGetCString(s, out, n, kCFStringEncodingUTF8)) {
			free(out);
			out = NULL;
		}
	}
	CFRelease(langs);
	return out;
}
*/
import "C"

import (
	"os"
	"unsafe"
)

// Detect 回傳系統介面語言（macOS：系統偏好語言清單的第一個）。
func Detect() string {
	c := C.firstPreferredLanguage()
	if c == nil {
		return detectFromEnv(os.Getenv)
	}
	defer C.free(unsafe.Pointer(c))
	return Normalize(C.GoString(c))
}
