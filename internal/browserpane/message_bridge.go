package browserpane

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"wails-app/internal/webresourcecache"
)

var (
	browserPaneMessageCallbackSeq atomic.Uint64
	browserPaneMessageCallbacks   sync.Map
)

func registerBrowserPaneMessageCallback(callback func(string)) uint64 {
	if callback == nil {
		return 0
	}
	id := browserPaneMessageCallbackSeq.Add(1)
	browserPaneMessageCallbacks.Store(id, callback)
	return id
}

func unregisterBrowserPaneMessageCallback(id uint64) {
	if id == 0 {
		return
	}
	browserPaneMessageCallbacks.Delete(id)
}

func dispatchBrowserPaneMessageCallback(id uint64, message string) {
	if id == 0 {
		return
	}
	value, ok := browserPaneMessageCallbacks.Load(id)
	if !ok {
		return
	}
	callback, ok := value.(func(string))
	if !ok || callback == nil {
		return
	}
	callback(message)
}

func handleBrowserPaneBridgeMessage(message string, onResourcesChanged func(string)) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	var snapshot struct {
		Type      string `json:"type"`
		PageURL   string `json:"pageUrl"`
		Resources []struct {
			URL           string `json:"url"`
			Size          int64  `json:"size"`
			InitiatorType string `json:"initiatorType"`
		} `json:"resources"`
	}
	if err := json.Unmarshal([]byte(message), &snapshot); err != nil {
		return
	}
	normalizedPageURL := normalizeBrowserPaneMessageURL(snapshot.PageURL)
	if snapshot.Type != "resource-snapshot" || normalizedPageURL == "" {
		return
	}

	resources := make([]webresourcecache.Resource, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		resources = append(resources, webresourcecache.Resource{
			URL:           resource.URL,
			Size:          resource.Size,
			InitiatorType: resource.InitiatorType,
			MimeType:      "",
		})
	}
	if webresourcecache.Store(normalizedPageURL, resources) && onResourcesChanged != nil {
		onResourcesChanged(normalizedPageURL)
	}
}

func normalizeBrowserPaneMessageURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.Path = strings.ReplaceAll(parsed.Path, "\\", "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	parsed.RawPath = ""
	return parsed.String()
}

func browserPaneInjectedResourceTrackerScript() string {
	return `(function(){
if(window.__wailsBrowserPaneResourceInit){return;}
window.__wailsBrowserPaneResourceInit=true;
function canonicalize(raw){
	try{
		var url=new URL(String(raw||''), location.href);
		url.hash='';
		return url.toString();
	}catch(_){return '';}
}
function post(message){
	try{
		if(window.chrome&&window.chrome.webview&&window.chrome.webview.postMessage){
			window.chrome.webview.postMessage(message);
			return true;
		}
	}catch(_){}
	try{
		if(window.webkit&&window.webkit.messageHandlers&&window.webkit.messageHandlers.wailsBrowserPane&&window.webkit.messageHandlers.wailsBrowserPane.postMessage){
			window.webkit.messageHandlers.wailsBrowserPane.postMessage(message);
			return true;
		}
	}catch(_){}
	return false;
}
function collect(){
	try{
		var resources=[];
		var seen=Object.create(null);
		var entries=(performance&&performance.getEntriesByType)?performance.getEntriesByType('resource'):[];
		for(var i=0;i<entries.length;i++){
			var entry=entries[i];
			var url=canonicalize(entry&&entry.name);
			if(!url||seen[url]){continue;}
			seen[url]=true;
			var transfer=Number(entry.transferSize)||0;
			var encoded=Number(entry.encodedBodySize)||0;
			var decoded=Number(entry.decodedBodySize)||0;
			var size=Math.max(transfer,encoded,decoded,0);
			resources.push({url:url,size:size,initiatorType:String(entry.initiatorType||'')});
		}
		post(JSON.stringify({
			type:'resource-snapshot',
			pageUrl:canonicalize(location.href),
			resources:resources
		}));
	}catch(_){ }
}
var scheduled=false;
function schedule(){
	if(scheduled){return;}
	scheduled=true;
	setTimeout(function(){scheduled=false;collect();},150);
}
window.__wailsBrowserPaneFlushResources=schedule;
if(window.PerformanceObserver){
	try{
		var observer=new PerformanceObserver(function(){schedule();});
		observer.observe({type:'resource',buffered:true});
	}catch(_){
		try{
			var fallbackObserver=new PerformanceObserver(function(){schedule();});
			fallbackObserver.observe({entryTypes:['resource']});
		}catch(__){}
	}
}
window.addEventListener('load',schedule,{once:false});
document.addEventListener('readystatechange',function(){if(document.readyState==='complete'){schedule();}});
setTimeout(schedule,500);
})();`
}
