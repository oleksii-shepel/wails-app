//go:build darwin && cgo

package browserpane

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>

@interface BrowserPaneController : NSObject
@property (nonatomic, retain) WKWebView *webView;
@property (nonatomic, assign) NSWindow *parentWindow;
@end

@implementation BrowserPaneController
@end

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
            NSRect frame = NSMakeRect((CGFloat)x, bounds.size.height - (CGFloat)y - (CGFloat)height, (CGFloat)width, (CGFloat)height);
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

            [pane.webView setHidden:visible ? NO : YES];
        }
    });
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
	"unsafe"
)

type darwinPane struct {
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
	return "", fmt.Errorf("capture screenshot not implemented on darwin")
}

func (p *darwinPane) GetDocumentBackground() (string, error) {
	return "", fmt.Errorf("document background query not implemented on darwin")
}

func (p *darwinPane) Dispose() error {
	C.browserPaneDispose(p.handle)
	p.handle = nil
	return nil
}
