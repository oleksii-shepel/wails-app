//go:build darwin && cgo

package browserpane

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <string.h>
#include <stdlib.h>

@interface BrowserPaneController : NSObject
@property (nonatomic, retain) WKWebView *webView;
@property (nonatomic, assign) NSWindow *parentWindow;
@end

@implementation BrowserPaneController
@end

static char *browserPaneCopyUTF8String(NSString *value) {
    if (!value) {
        return strdup("");
    }
    const char *utf8 = [value UTF8String];
    if (!utf8) {
        return strdup("");
    }
    return strdup(utf8);
}

static char *browserPaneCopyNSErrorMessage(NSError *error, const char *fallback) {
    if (error != nil && error.localizedDescription != nil) {
        return browserPaneCopyUTF8String(error.localizedDescription);
    }
    return strdup(fallback);
}

static void *browserPaneCreate(void) {
    __block BrowserPaneController *pane = nil;
    dispatch_sync(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            NSWindow *parent = [NSApp mainWindow];
            if (!parent) {
                parent = [NSApp keyWindow];
            }
            if (!parent) {
                return;
            }

            WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
            WKWebView *webView = [[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 1, 1) configuration:config];
            [webView setHidden:YES];

            pane = [[BrowserPaneController alloc] init];
            pane.parentWindow = parent;
            pane.webView = webView;

            NSView *contentView = [parent contentView];
            [contentView addSubview:webView positioned:NSWindowAbove relativeTo:nil];
        }
    });
    return (__bridge_retained void *)pane;
}

static void browserPaneSync(void *ptr, int x, int y, int width, int height, const char *url, int visible) {
    if (!ptr) {
        return;
    }
    dispatch_sync(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            BrowserPaneController *pane = (__bridge BrowserPaneController *)ptr;
            BOOL captureOnly = !visible && width > 0 && height > 0;
            if (!pane.parentWindow) {
                pane.parentWindow = [NSApp mainWindow];
                if (!pane.parentWindow) {
                    pane.parentWindow = [NSApp keyWindow];
                }
            }
            if (!pane.parentWindow || !pane.webView) {
                return;
            }

            NSView *contentView = [pane.parentWindow contentView];
            if ([pane.webView superview] != contentView) {
                [contentView addSubview:pane.webView positioned:NSWindowAbove relativeTo:nil];
            }

            NSRect bounds = [contentView bounds];
            NSRect frame = captureOnly
                ? NSMakeRect(-100000.0, -100000.0, (CGFloat)width, (CGFloat)height)
                : NSMakeRect((CGFloat)x, bounds.size.height - (CGFloat)y - (CGFloat)height, (CGFloat)width, (CGFloat)height);
            [pane.webView setFrame:frame];

            if (url != NULL && url[0] != '\0') {
                NSString *target = [NSString stringWithUTF8String:url];
                NSString *current = pane.webView.URL.absoluteString;
                if (!current || ![current isEqualToString:target]) {
                    NSURL *nsURL = [NSURL URLWithString:target];
                    if (nsURL) {
                        NSURLRequest *request = [NSURLRequest requestWithURL:nsURL];
                        [pane.webView loadRequest:request];
                    }
                }
            }

            [pane.webView setHidden:(visible || captureOnly) ? NO : YES];
        }
    });
}

static NSString *browserPaneDocumentBackgroundScript(void) {
    return @"(function(){"
           "  try {"
           "    var parseColor = function(value) {"
           "      if (!value || value === 'transparent') return null;"
           "      var match = value.match(/^rgba?\\(\\s*(\\d{1,3})\\s*,\\s*(\\d{1,3})\\s*,\\s*(\\d{1,3})(?:\\s*,\\s*([\\d.]+)\\s*)?\\)$/i);"
           "      if (!match) return null;"
           "      return {"
           "        r: Math.max(0, Math.min(255, parseInt(match[1], 10) || 0)),"
           "        g: Math.max(0, Math.min(255, parseInt(match[2], 10) || 0)),"
           "        b: Math.max(0, Math.min(255, parseInt(match[3], 10) || 0)),"
           "        a: Math.max(0, Math.min(1, match[4] == null ? 1 : parseFloat(match[4]) || 0))"
           "      };"
           "    };"
           "    var read = function(target) {"
           "      if (!target || !window.getComputedStyle) return '';"
           "      return window.getComputedStyle(target).backgroundColor || '';"
           "    };"
           "    var format = function(color) {"
           "      if (!color || color.a <= 0) return '';"
           "      if (color.a >= 1) return 'rgb(' + color.r + ', ' + color.g + ', ' + color.b + ')';"
           "      return 'rgba(' + color.r + ', ' + color.g + ', ' + color.b + ', ' + color.a + ')';"
           "    };"
           "    var composite = function(fg, bg) {"
           "      if (!fg) return bg;"
           "      if (!bg || fg.a >= 1) return fg;"
           "      if (fg.a <= 0) return bg;"
           "      var alpha = Math.max(0, Math.min(1, fg.a));"
           "      return {"
           "        r: Math.round(fg.r * alpha + bg.r * (1 - alpha)),"
           "        g: Math.round(fg.g * alpha + bg.g * (1 - alpha)),"
           "        b: Math.round(fg.b * alpha + bg.b * (1 - alpha)),"
           "        a: 1"
           "      };"
           "    };"
           "    var body = parseColor(read(document.body));"
           "    var root = parseColor(read(document.documentElement));"
           "    if (body && body.a > 0) {"
           "      return format(root && body.a < 1 ? composite(body, root) : body);"
           "    }"
           "    if (root && root.a > 0) {"
           "      return format(root);"
           "    }"
           "    return '';"
           "  } catch (_) {"
           "    return '';"
           "  }"
           "})()";
}

static void browserPaneCaptureScreenshot(void *ptr, char **outValue, char **outError) {
    if (outValue != NULL) {
        *outValue = NULL;
    }
    if (outError != NULL) {
        *outError = NULL;
    }
    if (!ptr) {
        if (outError != NULL) {
            *outError = strdup("browser pane not initialized");
        }
        return;
    }

    __block char *value = NULL;
    __block char *error = NULL;
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
    dispatch_async(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            BrowserPaneController *pane = (__bridge BrowserPaneController *)ptr;
            if (!pane.webView) {
                error = strdup("browser pane not initialized");
                dispatch_semaphore_signal(semaphore);
                return;
            }

            NSRect bounds = pane.webView.bounds;
            if (bounds.size.width <= 0 || bounds.size.height <= 0) {
                error = strdup("capture screenshot requires non-empty browser pane bounds");
                dispatch_semaphore_signal(semaphore);
                return;
            }

            WKSnapshotConfiguration *config = [[WKSnapshotConfiguration alloc] init];
            config.rect = bounds;
            [pane.webView takeSnapshotWithConfiguration:config completionHandler:^(NSImage *image, NSError *snapshotError) {
                @autoreleasepool {
                    if (snapshotError != nil) {
                        error = browserPaneCopyNSErrorMessage(snapshotError, "capture screenshot failed");
                        dispatch_semaphore_signal(semaphore);
                        return;
                    }
                    if (!image) {
                        error = strdup("capture screenshot returned no image");
                        dispatch_semaphore_signal(semaphore);
                        return;
                    }

                    NSData *tiffData = [image TIFFRepresentation];
                    if (!tiffData) {
                        error = strdup("capture screenshot returned empty image data");
                        dispatch_semaphore_signal(semaphore);
                        return;
                    }

                    NSBitmapImageRep *bitmap = [NSBitmapImageRep imageRepWithData:tiffData];
                    if (!bitmap) {
                        error = strdup("capture screenshot could not decode image");
                        dispatch_semaphore_signal(semaphore);
                        return;
                    }

                    NSData *pngData = [bitmap representationUsingType:NSPNGFileType properties:@{}];
                    if (!pngData) {
                        error = strdup("capture screenshot could not encode png");
                        dispatch_semaphore_signal(semaphore);
                        return;
                    }

                    NSString *base64 = [pngData base64EncodedStringWithOptions:0];
                    value = browserPaneCopyUTF8String(base64);
                    dispatch_semaphore_signal(semaphore);
                }
            }];
        }
    });

    dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
    if (outValue != NULL) {
        *outValue = value;
    }
    if (outError != NULL) {
        *outError = error;
    }
}

static void browserPaneGetDocumentBackground(void *ptr, char **outValue, char **outError) {
    if (outValue != NULL) {
        *outValue = NULL;
    }
    if (outError != NULL) {
        *outError = NULL;
    }
    if (!ptr) {
        if (outError != NULL) {
            *outError = strdup("browser pane not initialized");
        }
        return;
    }

    __block char *value = NULL;
    __block char *error = NULL;
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
    dispatch_async(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            BrowserPaneController *pane = (__bridge BrowserPaneController *)ptr;
            if (!pane.webView) {
                error = strdup("browser pane not initialized");
                dispatch_semaphore_signal(semaphore);
                return;
            }

            [pane.webView evaluateJavaScript:browserPaneDocumentBackgroundScript() completionHandler:^(id result, NSError *scriptError) {
                @autoreleasepool {
                    if (scriptError != nil) {
                        error = browserPaneCopyNSErrorMessage(scriptError, "document background query failed");
                        dispatch_semaphore_signal(semaphore);
                        return;
                    }

                    if ([result isKindOfClass:[NSString class]]) {
                        value = browserPaneCopyUTF8String((NSString *)result);
                    } else if (result != nil && [result respondsToSelector:@selector(stringValue)]) {
                        value = browserPaneCopyUTF8String([result stringValue]);
                    } else {
                        value = strdup("");
                    }
                    dispatch_semaphore_signal(semaphore);
                }
            }];
        }
    });

    dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
    if (outValue != NULL) {
        *outValue = value;
    }
    if (outError != NULL) {
        *outError = error;
    }
}

static void browserPaneFreeString(char *value) {
    if (value != NULL) {
        free(value);
    }
}

static void browserPaneDispose(void *ptr) {
    if (!ptr) {
        return;
    }
    dispatch_sync(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            BrowserPaneController *pane = (__bridge_transfer BrowserPaneController *)ptr;
            if (pane.webView) {
                [pane.webView removeFromSuperview];
                pane.webView = nil;
            }
            pane.parentWindow = nil;
        }
    });
}
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

type darwinPane struct {
	mu     sync.Mutex
	handle unsafe.Pointer
}

func newPlatformPane(_ Update, _ func(string)) (pane, error) {
	handle := C.browserPaneCreate()
	if handle == nil {
		return nil, fmt.Errorf("macOS main window not available")
	}
	return &darwinPane{handle: handle}, nil
}

func (p *darwinPane) Sync(update Update) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return fmt.Errorf("browser pane closed")
	}
	url := C.CString(update.URL)
	defer C.free(unsafe.Pointer(url))
	visible := 0
	if update.Visible {
		visible = 1
	}
	C.browserPaneSync(p.handle, C.int(update.X), C.int(update.Y), C.int(update.Width), C.int(update.Height), url, C.int(visible))
	return nil
}

func (p *darwinPane) CaptureScreenshot() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return "", fmt.Errorf("browser pane closed")
	}
	var value *C.char
	var captureErr *C.char
	C.browserPaneCaptureScreenshot(p.handle, &value, &captureErr)
	defer C.browserPaneFreeString(value)
	defer C.browserPaneFreeString(captureErr)
	if captureErr != nil {
		return "", fmt.Errorf("%s", C.GoString(captureErr))
	}
	if value == nil {
		return "", nil
	}
	return C.GoString(value), nil
}

func (p *darwinPane) GetDocumentBackground() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return "", fmt.Errorf("browser pane closed")
	}
	var value *C.char
	var backgroundErr *C.char
	C.browserPaneGetDocumentBackground(p.handle, &value, &backgroundErr)
	defer C.browserPaneFreeString(value)
	defer C.browserPaneFreeString(backgroundErr)
	if backgroundErr != nil {
		return "", fmt.Errorf("%s", C.GoString(backgroundErr))
	}
	if value == nil {
		return "", nil
	}
	return C.GoString(value), nil
}

func (p *darwinPane) Dispose() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return nil
	}
	C.browserPaneDispose(p.handle)
	p.handle = nil
	return nil
}
