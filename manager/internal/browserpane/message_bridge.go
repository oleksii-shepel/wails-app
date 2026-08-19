package browserpane

import (
	"encoding/json"
	"fmt"
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

func handleBrowserPaneBridgeMessage(message string, onResourcesChanged func(string), onMessage func(string)) {
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
	if err := json.Unmarshal([]byte(message), &snapshot); err == nil {
		normalizedPageURL := normalizeBrowserPaneMessageURL(snapshot.PageURL)
		if snapshot.Type == "resource-snapshot" && normalizedPageURL != "" {
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
			return
		}
	}

	if onMessage != nil {
		onMessage(message)
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

func browserPaneInjectedMessageBridgeScript() string {
	return `(function(){
if(window.__wailsBrowserPaneBridgeInit){return;}
window.__wailsBrowserPaneBridgeInit=true;
function sendRaw(message){
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
function serialize(payload){
	if(typeof payload==='string'){return payload;}
	try{
		var json=JSON.stringify(payload);
		return json===undefined?'null':json;
	}catch(_){}
	try{return String(payload);}catch(__){return '';}
}
function parse(raw){
	if(typeof raw!=='string'){return raw;}
	try{return JSON.parse(raw);}catch(_){return raw;}
}
function dispatch(raw){
	var payload=typeof raw==='string'?raw:serialize(raw);
	var detail=parse(payload);
	try{
		var event=null;
		if(typeof CustomEvent==='function'){
			event=new CustomEvent('wails-browser-pane-message',{detail:detail});
		}else if(document&&document.createEvent){
			event=document.createEvent('CustomEvent');
			event.initCustomEvent('wails-browser-pane-message',false,false,detail);
		}
		if(event){window.dispatchEvent(event);}
	}catch(_){}
	try{
		if(bridge&&typeof bridge.onmessage==='function'){
			bridge.onmessage({data:detail,rawData:payload});
		}
	}catch(_){}
	return detail;
}
var bridge=window.__wailsBrowserPane||{};
bridge.postMessage=function(payload){return sendRaw(serialize(payload));};
bridge.receiveHostMessage=function(raw){
	var detail=dispatch(raw);
	if(detail&&detail._rpc&&detail.id&&typeof detail.type!=='undefined'&&typeof bridge.onrequest==='function'){
		bridge.handleRequest(detail);
	}
	return detail;
};
bridge.parseMessage=parse;
if(!('onmessage' in bridge)){bridge.onmessage=null;}
if(!('onrequest' in bridge)){bridge.onrequest=null;}
bridge.handleRequest=async function(data){
	if(typeof bridge.onrequest!=='function')return;
	try{
		var result=await bridge.onrequest(data);
		bridge.postMessage({_rpc:true,id:data.id,result:result});
	}catch(err){
		bridge.postMessage({_rpc:true,id:data.id,error:(err&&err.message)?err.message:String(err)});
	}
};
window.__wailsBrowserPane=bridge;
})();`
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
		if(window.__wailsBrowserPane&&window.__wailsBrowserPane.postMessage){
			window.__wailsBrowserPane.postMessage({
			type:'resource-snapshot',
			pageUrl:canonicalize(location.href),
			resources:resources
			});
		}
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

func browserPaneInjectedLifecycleScript() string {
	return `(function(){
if(window.__wailsBrowserPaneLifecycleInit){return;}
window.__wailsBrowserPaneLifecycleInit=true;
function report(state){
	try{
		if(window.__wailsBrowserPane&&window.__wailsBrowserPane.postMessage){
			window.__wailsBrowserPane.postMessage({
				type:'lifecycle',
				state:state,
				url:location.href
			});
		}
	}catch(_){}
}
window.addEventListener('load',function(){report('completed');});
var _origPushState=history.pushState;
history.pushState=function(){
	_origPushState.apply(this,arguments);
	report('completed');
};
var _origReplaceState=history.replaceState;
history.replaceState=function(){
	_origReplaceState.apply(this,arguments);
	report('completed');
};
window.addEventListener('popstate',function(){report('completed');});
})();`
}

func browserPaneInjectedShortcutBridgeScript() string {
	return `(function(){
if(window.__wailsBrowserPaneShortcutInit){return;}
window.__wailsBrowserPaneShortcutInit=true;
function isFunctionKey(key){
	return /^F\d{1,2}$/i.test(String(key||''));
}
function shouldForward(event){
	if(!event||event.isComposing){return false;}
	if(event.ctrlKey||event.altKey||event.metaKey){return true;}
	return isFunctionKey(event.key);
}
window.addEventListener('keydown',function(event){
	if(!shouldForward(event)){return;}
	try{
		if(window.__wailsBrowserPane&&window.__wailsBrowserPane.postMessage){
			var target=event.target;
			var tagName=(target&&target.tagName)?String(target.tagName).toLowerCase():'';
			window.__wailsBrowserPane.postMessage({
				type:'shortcut',
				event:'keydown',
				key:String(event.key||''),
				code:String(event.code||''),
				ctrlKey:!!event.ctrlKey,
				shiftKey:!!event.shiftKey,
				altKey:!!event.altKey,
				metaKey:!!event.metaKey,
				context:{
					inInput:tagName==='input',
					inTextarea:tagName==='textarea',
					inSelect:tagName==='select',
					inContentEditable:!!(target&&target.isContentEditable)
				}
			});
		}
	}catch(_){}
},true);
})();`
}

func browserPaneInjectedBootstrapScript() string {
	return browserPaneInjectedMessageBridgeScript() + "\n" + browserPaneInjectedResourceTrackerScript() + "\n" + browserPaneInjectedLifecycleScript() + "\n" + browserPaneInjectedShortcutBridgeScript()
}

func browserPaneHostMessageEvalScript(message string) string {
	encoded, err := json.Marshal(message)
	if err != nil {
		encoded = []byte(`""`)
	}
	return fmt.Sprintf(`(function(){
var raw=%s;
var bridge=window.__wailsBrowserPane;
if(bridge&&typeof bridge.receiveHostMessage==='function'){
	bridge.receiveHostMessage(raw);
	return true;
}
var detail=raw;
try{detail=JSON.parse(raw);}catch(_){}
try{
	var event=null;
	if(typeof CustomEvent==='function'){
		event=new CustomEvent('wails-browser-pane-message',{detail:detail});
	}else if(document&&document.createEvent){
		event=document.createEvent('CustomEvent');
		event.initCustomEvent('wails-browser-pane-message',false,false,detail);
	}
	if(event){window.dispatchEvent(event);}
	return true;
}catch(_){
	return false;
}
})();`, string(encoded))
}