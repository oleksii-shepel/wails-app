//go:build linux && cgo

package browserpane

/*
#cgo linux pkg-config: gtk+-3.0
#cgo !webkit2_41 pkg-config: webkit2gtk-4.0
#cgo webkit2_41 pkg-config: webkit2gtk-4.1

#include <stdint.h>
#include <gdk-pixbuf/gdk-pixbuf.h>
#include <gtk/gtk.h>
#include <jsc/jsc.h>
#include <webkit2/webkit2.h>
#include <stdlib.h>

extern void browserPaneLinuxHandleScriptMessage(unsigned long long callback_id, const char *message);

typedef struct BrowserPaneController {
	GtkWindow *window;
	WebKitWebView *webview;
	GtkWindow *parent;
	uint64_t callback_id;
	GPtrArray *pending_scripts;
	gboolean page_ready;
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

typedef struct BrowserPaneStringArgs {
	BrowserPaneController *pane;
	char *value;
	char *error;
	GMutex mutex;
	GCond cond;
	gboolean done;
} BrowserPaneStringArgs;

typedef struct BrowserPaneScriptArgs {
	BrowserPaneController *pane;
	char *script;
	char *error;
	GMutex mutex;
	GCond cond;
	gboolean done;
} BrowserPaneScriptArgs;

static void browser_pane_signal_done(GMutex *mutex, GCond *cond, gboolean *done) {
	g_mutex_lock(mutex);
	*done = TRUE;
	g_cond_signal(cond);
	g_mutex_unlock(mutex);
}

static void browser_pane_signal_string_result(BrowserPaneStringArgs *args, char *value, char *error) {
	if (args == NULL) {
		return;
	}
	args->value = value;
	args->error = error;
	browser_pane_signal_done(&args->mutex, &args->cond, &args->done);
}

static void browser_pane_signal_string_error(BrowserPaneStringArgs *args, const char *message) {
	browser_pane_signal_string_result(args, NULL, g_strdup(message));
}

static void browser_pane_signal_script_done(BrowserPaneScriptArgs *args, char *error) {
	if (args == NULL) {
		return;
	}
	args->error = error;
	browser_pane_signal_done(&args->mutex, &args->cond, &args->done);
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

static void browser_pane_script_message_received(WebKitUserContentManager *manager, WebKitJavascriptResult *result, gpointer user_data) {
	(void)manager;
	BrowserPaneController *pane = (BrowserPaneController *)user_data;
	if (pane == NULL || pane->callback_id == 0) {
		return;
	}
	JSCValue *value = webkit_javascript_result_get_js_value(result);
	if (value == NULL || jsc_value_is_null(value) || jsc_value_is_undefined(value)) {
		return;
	}
	gchar *text = jsc_value_to_string(value);
	if (text == NULL || text[0] == '\0') {
		g_free(text);
		return;
	}
	browserPaneLinuxHandleScriptMessage((unsigned long long)pane->callback_id, text);
	g_free(text);
}

typedef struct BrowserPaneExecuteScriptData {
	char *script;
} BrowserPaneExecuteScriptData;

static void browser_pane_execute_script_finished(GObject *source_object, GAsyncResult *result, gpointer user_data) {
	BrowserPaneExecuteScriptData *data = (BrowserPaneExecuteScriptData *)user_data;
	GError *error = NULL;
	webkit_web_view_run_javascript_finish(WEBKIT_WEB_VIEW(source_object), result, &error);
	if (error != NULL) {
		g_error_free(error);
	}
	if (data != NULL) {
		g_free(data->script);
		g_free(data);
	}
}

static void browser_pane_queue_or_execute_script(BrowserPaneController *pane, const char *script) {
	if (pane == NULL || pane->webview == NULL || script == NULL || script[0] == '\0') {
		return;
	}
	if (!pane->page_ready) {
		if (pane->pending_scripts == NULL) {
			pane->pending_scripts = g_ptr_array_new_with_free_func(g_free);
		}
		g_ptr_array_add(pane->pending_scripts, g_strdup(script));
		return;
	}
	BrowserPaneExecuteScriptData *data = g_malloc0(sizeof(BrowserPaneExecuteScriptData));
	data->script = g_strdup(script);
	webkit_web_view_run_javascript(
		pane->webview,
		data->script,
		NULL,
		browser_pane_execute_script_finished,
		data
	);
}

static void browser_pane_flush_pending_scripts(BrowserPaneController *pane) {
	if (pane == NULL || pane->pending_scripts == NULL || pane->pending_scripts->len == 0) {
		return;
	}
	for (guint i = 0; i < pane->pending_scripts->len; i++) {
		char *script = (char *)g_ptr_array_index(pane->pending_scripts, i);
		browser_pane_queue_or_execute_script(pane, script);
	}
	g_ptr_array_set_size(pane->pending_scripts, 0);
}

static void browser_pane_load_changed(WebKitWebView *webview, WebKitLoadEvent load_event, gpointer user_data) {
	(void)webview;
	BrowserPaneController *pane = (BrowserPaneController *)user_data;
	if (pane == NULL) {
		return;
	}
	switch (load_event) {
	case WEBKIT_LOAD_STARTED:
	case WEBKIT_LOAD_REDIRECTED:
	case WEBKIT_LOAD_COMMITTED:
		pane->page_ready = FALSE;
		break;
	case WEBKIT_LOAD_FINISHED:
		pane->page_ready = TRUE;
		browser_pane_flush_pending_scripts(pane);
		break;
	default:
		break;
	}
}

static gboolean browser_pane_create_on_main(gpointer data) {
	BrowserPaneCreateArgs *args = (BrowserPaneCreateArgs *)data;
	BrowserPaneController *pane = g_malloc0(sizeof(BrowserPaneController));
	pane->parent = browser_pane_discover_main_window();
	if (pane->parent != NULL) {
		WebKitUserContentManager *manager = webkit_user_content_manager_new();
		if (manager != NULL) {
			webkit_user_content_manager_register_script_message_handler(manager, "wailsBrowserPane");
			g_signal_connect(manager, "script-message-received::wailsBrowserPane", G_CALLBACK(browser_pane_script_message_received), pane);
		}
		GtkWidget *window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
		gtk_window_set_decorated(GTK_WINDOW(window), FALSE);
		gtk_window_set_resizable(GTK_WINDOW(window), FALSE);
		gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window), TRUE);
		gtk_window_set_skip_pager_hint(GTK_WINDOW(window), TRUE);
		gtk_window_set_type_hint(GTK_WINDOW(window), GDK_WINDOW_TYPE_HINT_UTILITY);
		gtk_window_set_transient_for(GTK_WINDOW(window), pane->parent);
		gtk_window_set_destroy_with_parent(GTK_WINDOW(window), TRUE);

		GtkWidget *webview = manager != NULL
			? webkit_web_view_new_with_user_content_manager(manager)
			: webkit_web_view_new();
		if (manager != NULL) {
			g_object_unref(manager);
		}
		gtk_container_add(GTK_CONTAINER(window), webview);
		gtk_widget_show(webview);
		gtk_widget_hide(window);

		pane->window = GTK_WINDOW(window);
		pane->webview = WEBKIT_WEB_VIEW(webview);
		pane->pending_scripts = g_ptr_array_new_with_free_func(g_free);
		pane->page_ready = FALSE;
		g_signal_connect(webview, "load-changed", G_CALLBACK(browser_pane_load_changed), pane);
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

static BrowserPaneController *browser_pane_create(uint64_t callback_id, const char *resource_script) {
	BrowserPaneCreateArgs args;
	args.pane = NULL;
	args.done = FALSE;
	g_mutex_init(&args.mutex);
	g_cond_init(&args.cond);
	g_main_context_invoke(NULL, browser_pane_create_on_main, &args);
	browser_pane_wait_for_completion(&args.mutex, &args.cond, &args.done);
	g_cond_clear(&args.cond);
	g_mutex_clear(&args.mutex);
	if (args.pane != NULL) {
		args.pane->callback_id = callback_id;
		if (resource_script != NULL && resource_script[0] != '\0') {
			WebKitUserContentManager *manager = webkit_web_view_get_user_content_manager(args.pane->webview);
			if (manager != NULL) {
				WebKitUserScript *script = webkit_user_script_new(
					resource_script,
					WEBKIT_USER_CONTENT_INJECT_ALL_FRAMES,
					WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START,
					NULL,
					NULL
				);
				if (script != NULL) {
					webkit_user_content_manager_add_script(manager, script);
					webkit_user_script_unref(script);
				}
			}
		}
	}
	return args.pane;
}

static gboolean browser_pane_sync_on_main(gpointer data) {
	BrowserPaneSyncArgs *args = (BrowserPaneSyncArgs *)data;
	BrowserPaneController *pane = args->pane;
	if (pane != NULL && pane->parent != NULL && pane->window != NULL && pane->webview != NULL) {
		gboolean capture_only = !args->visible && args->width > 0 && args->height > 0;
		if (args->visible || capture_only) {
			GdkWindow *root = gtk_widget_get_window(GTK_WIDGET(pane->parent));
			if (capture_only) {
				gtk_window_move(pane->window, -32768, -32768);
				gtk_window_resize(pane->window, args->width, args->height);
			} else if (root != NULL) {
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
		if (args->pane->pending_scripts != NULL) {
			g_ptr_array_free(args->pane->pending_scripts, TRUE);
			args->pane->pending_scripts = NULL;
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

static void browser_pane_free_string(char *value) {
	if (value != NULL) {
		g_free(value);
	}
}

static void browser_pane_snapshot_finished(GObject *source_object, GAsyncResult *result, gpointer user_data) {
	BrowserPaneStringArgs *args = (BrowserPaneStringArgs *)user_data;
	GError *error = NULL;
	cairo_surface_t *surface = webkit_web_view_get_snapshot_finish(WEBKIT_WEB_VIEW(source_object), result, &error);
	if (surface == NULL) {
		char *message = g_strdup(error != NULL ? error->message : "capture screenshot failed");
		if (error != NULL) {
			g_error_free(error);
		}
		browser_pane_signal_string_result(args, NULL, message);
		return;
	}

	int width = gtk_widget_get_allocated_width(GTK_WIDGET(args->pane->webview));
	int height = gtk_widget_get_allocated_height(GTK_WIDGET(args->pane->webview));
	if (width <= 0 || height <= 0) {
		cairo_surface_destroy(surface);
		browser_pane_signal_string_error(args, "capture screenshot requires non-empty browser pane bounds");
		return;
	}

	GdkPixbuf *pixbuf = gdk_pixbuf_get_from_surface(surface, 0, 0, width, height);
	cairo_surface_destroy(surface);
	if (pixbuf == NULL) {
		browser_pane_signal_string_error(args, "capture screenshot could not read surface pixels");
		return;
	}

	gchar *png_bytes = NULL;
	gsize png_size = 0;
	if (!gdk_pixbuf_save_to_buffer(pixbuf, &png_bytes, &png_size, "png", &error, NULL)) {
		g_object_unref(pixbuf);
		char *message = g_strdup(error != NULL ? error->message : "capture screenshot could not encode png");
		if (error != NULL) {
			g_error_free(error);
		}
		browser_pane_signal_string_result(args, NULL, message);
		return;
	}
	g_object_unref(pixbuf);

	char *encoded = (char *)g_base64_encode((const guchar *)png_bytes, png_size);
	g_free(png_bytes);
	if (encoded == NULL) {
		browser_pane_signal_string_error(args, "capture screenshot returned no data");
		return;
	}
	browser_pane_signal_string_result(args, encoded, NULL);
}

static gboolean browser_pane_capture_screenshot_on_main(gpointer data) {
	BrowserPaneStringArgs *args = (BrowserPaneStringArgs *)data;
	if (args == NULL || args->pane == NULL || args->pane->webview == NULL) {
		browser_pane_signal_string_error(args, "browser pane not initialized");
		return G_SOURCE_REMOVE;
	}

	int width = gtk_widget_get_allocated_width(GTK_WIDGET(args->pane->webview));
	int height = gtk_widget_get_allocated_height(GTK_WIDGET(args->pane->webview));
	if (width <= 0 || height <= 0) {
		browser_pane_signal_string_error(args, "capture screenshot requires non-empty browser pane bounds");
		return G_SOURCE_REMOVE;
	}

	webkit_web_view_get_snapshot(
		args->pane->webview,
		WEBKIT_SNAPSHOT_REGION_VISIBLE,
		WEBKIT_SNAPSHOT_OPTIONS_NONE,
		NULL,
		browser_pane_snapshot_finished,
		args
	);
	return G_SOURCE_REMOVE;
}

static const gchar *browser_pane_document_background_script =
	"(function(){"
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

static void browser_pane_background_finished(GObject *source_object, GAsyncResult *result, gpointer user_data) {
	BrowserPaneStringArgs *args = (BrowserPaneStringArgs *)user_data;
	GError *error = NULL;
	WebKitJavascriptResult *script_result = webkit_web_view_run_javascript_finish(WEBKIT_WEB_VIEW(source_object), result, &error);
	if (script_result == NULL) {
		char *message = g_strdup(error != NULL ? error->message : "document background query failed");
		if (error != NULL) {
			g_error_free(error);
		}
		browser_pane_signal_string_result(args, NULL, message);
		return;
	}

	JSCValue *value = webkit_javascript_result_get_js_value(script_result);
	gchar *text = g_strdup("");
	if (value != NULL && !jsc_value_is_null(value) && !jsc_value_is_undefined(value)) {
		gchar *converted = jsc_value_to_string(value);
		if (converted != NULL) {
			g_free(text);
			text = converted;
		}
	}

	webkit_javascript_result_unref(script_result);
	browser_pane_signal_string_result(args, text, NULL);
}

static gboolean browser_pane_get_document_background_on_main(gpointer data) {
	BrowserPaneStringArgs *args = (BrowserPaneStringArgs *)data;
	if (args == NULL || args->pane == NULL || args->pane->webview == NULL) {
		browser_pane_signal_string_error(args, "browser pane not initialized");
		return G_SOURCE_REMOVE;
	}

	webkit_web_view_run_javascript(
		args->pane->webview,
		browser_pane_document_background_script,
		NULL,
		browser_pane_background_finished,
		args
	);
	return G_SOURCE_REMOVE;
}

static void browser_pane_capture_screenshot(BrowserPaneController *pane, char **value, char **error) {
	BrowserPaneStringArgs args;
	args.pane = pane;
	args.value = NULL;
	args.error = NULL;
	args.done = FALSE;
	g_mutex_init(&args.mutex);
	g_cond_init(&args.cond);
	g_main_context_invoke(NULL, browser_pane_capture_screenshot_on_main, &args);
	browser_pane_wait_for_completion(&args.mutex, &args.cond, &args.done);
	g_cond_clear(&args.cond);
	g_mutex_clear(&args.mutex);
	if (value != NULL) {
		*value = args.value;
	}
	if (error != NULL) {
		*error = args.error;
	}
}

static void browser_pane_get_document_background(BrowserPaneController *pane, char **value, char **error) {
	BrowserPaneStringArgs args;
	args.pane = pane;
	args.value = NULL;
	args.error = NULL;
	args.done = FALSE;
	g_mutex_init(&args.mutex);
	g_cond_init(&args.cond);
	g_main_context_invoke(NULL, browser_pane_get_document_background_on_main, &args);
	browser_pane_wait_for_completion(&args.mutex, &args.cond, &args.done);
	g_cond_clear(&args.cond);
	g_mutex_clear(&args.mutex);
	if (value != NULL) {
		*value = args.value;
	}
	if (error != NULL) {
		*error = args.error;
	}
}

static gboolean browser_pane_post_message_on_main(gpointer data) {
	BrowserPaneScriptArgs *args = (BrowserPaneScriptArgs *)data;
	if (args == NULL || args->pane == NULL || args->pane->webview == NULL) {
		browser_pane_signal_script_done(args, g_strdup("browser pane not initialized"));
		return G_SOURCE_REMOVE;
	}
	if (args->script == NULL || args->script[0] == '\0') {
		browser_pane_signal_script_done(args, NULL);
		return G_SOURCE_REMOVE;
	}
	browser_pane_queue_or_execute_script(args->pane, args->script);
	browser_pane_signal_script_done(args, NULL);
	return G_SOURCE_REMOVE;
}

static void browser_pane_post_message(BrowserPaneController *pane, const char *script, char **error) {
	BrowserPaneScriptArgs args;
	args.pane = pane;
	args.script = (char *)script;
	args.error = NULL;
	args.done = FALSE;
	g_mutex_init(&args.mutex);
	g_cond_init(&args.cond);
	g_main_context_invoke(NULL, browser_pane_post_message_on_main, &args);
	browser_pane_wait_for_completion(&args.mutex, &args.cond, &args.done);
	g_cond_clear(&args.cond);
	g_mutex_clear(&args.mutex);
	if (error != NULL) {
		*error = args.error;
	}
}
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

type linuxPane struct {
	mu                sync.Mutex
	handle            *C.BrowserPaneController
	messageCallbackID uint64
}

func newPlatformPane(_ Update, onResourcesChanged func(string), onMessage func(string)) (pane, error) {
	messageCallbackID := registerBrowserPaneMessageCallback(func(message string) {
		handleBrowserPaneBridgeMessage(message, onResourcesChanged, onMessage)
	})
	resourceScript := C.CString(browserPaneInjectedBootstrapScript())
	defer C.free(unsafe.Pointer(resourceScript))

	handle := C.browser_pane_create(C.ulonglong(messageCallbackID), resourceScript)
	if handle == nil {
		unregisterBrowserPaneMessageCallback(messageCallbackID)
		return nil, fmt.Errorf("linux main window not available")
	}
	return &linuxPane{handle: handle, messageCallbackID: messageCallbackID}, nil
}

func (p *linuxPane) Sync(update Update) error {
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
	C.browser_pane_sync(p.handle, C.int(update.X), C.int(update.Y), C.int(update.Width), C.int(update.Height), url, C.int(visible))
	return nil
}

func (p *linuxPane) CaptureScreenshot() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return "", fmt.Errorf("browser pane closed")
	}
	var value *C.char
	var captureErr *C.char
	C.browser_pane_capture_screenshot(p.handle, &value, &captureErr)
	defer C.browser_pane_free_string(value)
	defer C.browser_pane_free_string(captureErr)
	if captureErr != nil {
		return "", fmt.Errorf("%s", C.GoString(captureErr))
	}
	if value == nil {
		return "", nil
	}
	return C.GoString(value), nil
}

func (p *linuxPane) GetDocumentBackground() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return "", fmt.Errorf("browser pane closed")
	}
	var value *C.char
	var backgroundErr *C.char
	C.browser_pane_get_document_background(p.handle, &value, &backgroundErr)
	defer C.browser_pane_free_string(value)
	defer C.browser_pane_free_string(backgroundErr)
	if backgroundErr != nil {
		return "", fmt.Errorf("%s", C.GoString(backgroundErr))
	}
	if value == nil {
		return "", nil
	}
	return C.GoString(value), nil
}

func (p *linuxPane) PostMessage(message string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return fmt.Errorf("browser pane closed")
	}
	script := C.CString(browserPaneHostMessageEvalScript(message))
	defer C.free(unsafe.Pointer(script))
	var postErr *C.char
	C.browser_pane_post_message(p.handle, script, &postErr)
	defer C.browser_pane_free_string(postErr)
	if postErr != nil {
		return fmt.Errorf("%s", C.GoString(postErr))
	}
	return nil
}

func (p *linuxPane) Dispose() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		unregisterBrowserPaneMessageCallback(p.messageCallbackID)
		p.messageCallbackID = 0
		return nil
	}
	C.browser_pane_dispose(p.handle)
	p.handle = nil
	unregisterBrowserPaneMessageCallback(p.messageCallbackID)
	p.messageCallbackID = 0
	return nil
}
