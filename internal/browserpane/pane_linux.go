//go:build linux && cgo

package browserpane

/*
#cgo linux pkg-config: gtk+-3.0
#cgo !webkit2_41 pkg-config: webkit2gtk-4.0
#cgo webkit2_41 pkg-config: webkit2gtk-4.1

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
#include <stdlib.h>

typedef struct BrowserPaneController {
	GtkWindow *window;
	WebKitWebView *webview;
	GtkWindow *parent;
} BrowserPaneController;

typedef struct BrowserPaneCreateArgs {
	BrowserPaneController *pane;
	GMutex mutex;
	GCond cond;
	gboolean done;
} BrowserPaneCreateArgs;

typedef struct BrowserPaneSyncArgs {
	BrowserPaneController *pane;
	int x;
	int y;
	int width;
	int height;
	int visible;
	char *url;
	GMutex mutex;
	GCond cond;
	gboolean done;
} BrowserPaneSyncArgs;

typedef struct BrowserPaneDisposeArgs {
	BrowserPaneController *pane;
	GMutex mutex;
	GCond cond;
	gboolean done;
} BrowserPaneDisposeArgs;

static void browser_pane_signal_done(GMutex *mutex, GCond *cond, gboolean *done) {
	g_mutex_lock(mutex);
	*done = TRUE;
	g_cond_signal(cond);
	g_mutex_unlock(mutex);
}

static GtkWindow *browser_pane_discover_main_window(void) {
	GList *windows = gtk_window_list_toplevels();
	for (GList *it = windows; it != NULL; it = it->next) {
		if (!GTK_IS_WINDOW(it->data)) {
			continue;
		}
		GtkWindow *candidate = GTK_WINDOW(it->data);
		if (!gtk_widget_get_visible(GTK_WIDGET(candidate))) {
			continue;
		}
		return candidate;
	}
	return NULL;
}

static gboolean browser_pane_create_on_main(gpointer data) {
	BrowserPaneCreateArgs *args = (BrowserPaneCreateArgs *)data;
	BrowserPaneController *pane = g_malloc0(sizeof(BrowserPaneController));
	pane->parent = browser_pane_discover_main_window();
	if (pane->parent != NULL) {
		GtkWidget *window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
		gtk_window_set_decorated(GTK_WINDOW(window), FALSE);
		gtk_window_set_resizable(GTK_WINDOW(window), FALSE);
		gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window), TRUE);
		gtk_window_set_skip_pager_hint(GTK_WINDOW(window), TRUE);
		gtk_window_set_type_hint(GTK_WINDOW(window), GDK_WINDOW_TYPE_HINT_UTILITY);
		gtk_window_set_transient_for(GTK_WINDOW(window), pane->parent);
		gtk_window_set_destroy_with_parent(GTK_WINDOW(window), TRUE);

		GtkWidget *webview = webkit_web_view_new();
		gtk_container_add(GTK_CONTAINER(window), webview);
		gtk_widget_show(webview);
		gtk_widget_hide(window);

		pane->window = GTK_WINDOW(window);
		pane->webview = WEBKIT_WEB_VIEW(webview);
	}
	args->pane = pane;
	browser_pane_signal_done(&args->mutex, &args->cond, &args->done);
	return G_SOURCE_REMOVE;
}

static void browser_pane_wait_for_completion(GMutex *mutex, GCond *cond, gboolean *done) {
	g_mutex_lock(mutex);
	while (!*done) {
		g_cond_wait(cond, mutex);
	}
	g_mutex_unlock(mutex);
}

static BrowserPaneController *browser_pane_create(void) {
	BrowserPaneCreateArgs args;
	args.pane = NULL;
	args.done = FALSE;
	g_mutex_init(&args.mutex);
	g_cond_init(&args.cond);
	g_main_context_invoke(NULL, browser_pane_create_on_main, &args);
	browser_pane_wait_for_completion(&args.mutex, &args.cond, &args.done);
	g_cond_clear(&args.cond);
	g_mutex_clear(&args.mutex);
	return args.pane;
}

static gboolean browser_pane_sync_on_main(gpointer data) {
	BrowserPaneSyncArgs *args = (BrowserPaneSyncArgs *)data;
	BrowserPaneController *pane = args->pane;
	if (pane != NULL && pane->parent != NULL && pane->window != NULL && pane->webview != NULL) {
		if (args->visible) {
			GdkWindow *root = gtk_widget_get_window(GTK_WIDGET(pane->parent));
			if (root != NULL) {
				int originX = 0;
				int originY = 0;
				gdk_window_get_origin(root, &originX, &originY);
				gtk_window_move(pane->window, originX + args->x, originY + args->y);
				gtk_window_resize(pane->window, args->width, args->height);
			}
			if (args->url != NULL && args->url[0] != '\0') {
				const gchar *current = webkit_web_view_get_uri(pane->webview);
				if (current == NULL || g_strcmp0(current, args->url) != 0) {
					webkit_web_view_load_uri(pane->webview, args->url);
				}
			}
			gtk_widget_show_all(GTK_WIDGET(pane->window));
		} else {
			gtk_widget_hide(GTK_WIDGET(pane->window));
		}
	}
	browser_pane_signal_done(&args->mutex, &args->cond, &args->done);
	return G_SOURCE_REMOVE;
}

static void browser_pane_sync(BrowserPaneController *pane, int x, int y, int width, int height, const char *url, int visible) {
	BrowserPaneSyncArgs args;
	args.pane = pane;
	args.x = x;
	args.y = y;
	args.width = width;
	args.height = height;
	args.visible = visible;
	args.url = (char *)url;
	args.done = FALSE;
	g_mutex_init(&args.mutex);
	g_cond_init(&args.cond);
	g_main_context_invoke(NULL, browser_pane_sync_on_main, &args);
	browser_pane_wait_for_completion(&args.mutex, &args.cond, &args.done);
	g_cond_clear(&args.cond);
	g_mutex_clear(&args.mutex);
}

static gboolean browser_pane_dispose_on_main(gpointer data) {
	BrowserPaneDisposeArgs *args = (BrowserPaneDisposeArgs *)data;
	if (args->pane != NULL) {
		if (args->pane->window != NULL) {
			gtk_widget_destroy(GTK_WIDGET(args->pane->window));
		}
		g_free(args->pane);
	}
	browser_pane_signal_done(&args->mutex, &args->cond, &args->done);
	return G_SOURCE_REMOVE;
}

static void browser_pane_dispose(BrowserPaneController *pane) {
	BrowserPaneDisposeArgs args;
	args.pane = pane;
	args.done = FALSE;
	g_mutex_init(&args.mutex);
	g_cond_init(&args.cond);
	g_main_context_invoke(NULL, browser_pane_dispose_on_main, &args);
	browser_pane_wait_for_completion(&args.mutex, &args.cond, &args.done);
	g_cond_clear(&args.cond);
	g_mutex_clear(&args.mutex);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type linuxPane struct {
	handle *C.BrowserPaneController
}

func newPlatformPane(_ Update, _ func(string)) (pane, error) {
	handle := C.browser_pane_create()
	if handle == nil {
		return nil, fmt.Errorf("linux main window not available")
	}
	return &linuxPane{handle: handle}, nil
}

func (p *linuxPane) Sync(update Update) error {
	url := C.CString(update.URL)
	defer C.free(unsafe.Pointer(url))
	visible := 0
	if update.Visible {
		visible = 1
	}
	C.browser_pane_sync(p.handle, C.int(update.X), C.int(update.Y), C.int(update.Width), C.int(update.Height), url, C.int(visible))
	return nil
}

func (p *linuxPane) CaptureScreenshot() (string, error) {
	return "", fmt.Errorf("capture screenshot not implemented on linux")
}

func (p *linuxPane) GetDocumentBackground() (string, error) {
	return "", fmt.Errorf("document background query not implemented on linux")
}

func (p *linuxPane) Dispose() error {
	C.browser_pane_dispose(p.handle)
	p.handle = nil
	return nil
}
